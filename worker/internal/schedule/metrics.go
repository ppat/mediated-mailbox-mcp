package schedule

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// The worker's own series, one set per job kind, labelled by the job kind as job_runs.workload
// spells it and, for a job's own series, by the account the job serves, empty for a job of the whole
// kind (ADR-0076, ADR-0119). The alerting rules in packaging/chart/alerting-rules.yaml read
// lastSuccessName, lateName, waitedName and waitingName, so they stay exactly as written (ADR-0077).
const (
	startedName     = "mediated_mailbox_job_runs_started_total"
	finishedName    = "mediated_mailbox_job_runs_finished_total"
	durationName    = "mediated_mailbox_job_run_duration_seconds"
	runningName     = "mediated_mailbox_job_runs_running"
	backoffName     = "mediated_mailbox_job_backoff_seconds"
	lastSuccessName = "mediated_mailbox_job_last_success_timestamp_seconds"
	lateName        = "mediated_mailbox_job_late_after_seconds"
	waitedName      = "mediated_mailbox_job_slot_waited_seconds"
	waitingName     = "mediated_mailbox_job_slot_waiting_since_timestamp_seconds"
)

// Metrics are the scheduler's series on one registry. They implement Observer.
type Metrics struct {
	started     *prometheus.CounterVec
	finished    *prometheus.CounterVec
	duration    *prometheus.HistogramVec
	running     *prometheus.GaugeVec
	backoff     *prometheus.GaugeVec
	lastSuccess *prometheus.GaugeVec
	late        *prometheus.GaugeVec
	waited      *prometheus.GaugeVec
	waiting     *prometheus.GaugeVec
}

var _ Observer = (*Metrics)(nil)

// NewMetrics registers the scheduler's series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	job := []string{"job_kind", "account"}
	m := &Metrics{
		started: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: startedName,
			Help: "Runs of the job kind's jobs started, every ask the scheduler made of a job.",
		}, []string{"job_kind"}),
		finished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: finishedName,
			Help: "Runs of the job kind's jobs finished, by how each ended: succeeded, failed, panicked, not_due or cancelled.",
		}, []string{"job_kind", "outcome"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    durationName,
			Help:    "How long the job kind's runs that succeeded, failed or panicked took, from their start to their return, by job, reload for the job of the whole kind and account for an account's.",
			Buckets: []float64{0.01, 0.1, 1, 10, 60, 300, 1800, 3600, 4 * 3600, 12 * 3600},
		}, []string{"job_kind", "job"}),
		running: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: runningName,
			Help: "Runs of the job kind's jobs running now.",
		}, []string{"job_kind"}),
		backoff: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: backoffName,
			Help: "The delay the job waits after its latest failed run before it is asked again, and 0 while it waits for none.",
		}, job),
		lastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: lastSuccessName,
			Help: "The time of the job's latest success, a run that succeeded or a unit of work it made durable, starting from the one its job kind's runs record, or from the job's ensure when they record none, in seconds since the epoch.",
		}, job),
		late: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: lateName,
			Help: "How long after its latest success the job counts as late, in seconds, not counting the time it waited for a slot of its kind's limit.",
		}, job),
		waited: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: waitedName,
			Help: "How long the job has waited for a slot of its kind's limit since its latest success, over the waits that have ended, in seconds.",
		}, job),
		waiting: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: waitingName,
			Help: "When the job's wait for a slot of its kind's limit began, while it waits, in seconds since the epoch, and 0 while it waits for none.",
		}, job),
	}
	for _, c := range []prometheus.Collector{m.started, m.finished, m.duration, m.running, m.backoff, m.lastSuccess, m.late, m.waited, m.waiting} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Ensured implements Observer. A job's series start at its ensure, its latest success at the one its
// job kind's recorded state holds, or the ensure when it holds none, so a job that never succeeds
// ages from there.
func (m *Metrics) Ensured(key Key, late time.Duration, at time.Time) {
	m.running.WithLabelValues(key.Kind).Add(0)
	m.backoff.WithLabelValues(key.Kind, key.ID).Set(0)
	m.lastSuccess.WithLabelValues(key.Kind, key.ID).Set(seconds(at))
	m.late.WithLabelValues(key.Kind, key.ID).Set(late.Seconds())
	m.waited.WithLabelValues(key.Kind, key.ID).Set(0)
	m.waiting.WithLabelValues(key.Kind, key.ID).Set(0)
}

// Waiting implements Observer.
func (m *Metrics) Waiting(key Key, since time.Time) {
	m.waiting.WithLabelValues(key.Kind, key.ID).Set(seconds(since))
}

// Waited implements Observer. The wait joins the time the job has waited since its latest success.
func (m *Metrics) Waited(key Key, since, until time.Time) {
	m.waited.WithLabelValues(key.Kind, key.ID).Add(until.Sub(since).Seconds())
	m.waiting.WithLabelValues(key.Kind, key.ID).Set(0)
}

// Started implements Observer.
func (m *Metrics) Started(key Key) {
	m.started.WithLabelValues(key.Kind).Inc()
	m.running.WithLabelValues(key.Kind).Inc()
}

// Finished implements Observer. Every outcome is counted, and only a run that did its work, one that
// succeeded, failed or panicked, is timed, by its job, so an instant not_due run or a cancelled one
// does not pull the distribution down and a reload's time is told apart from an account job's
// (ADR-0125).
func (m *Metrics) Finished(key Key, outcome Outcome, took time.Duration) {
	m.running.WithLabelValues(key.Kind).Dec()
	m.finished.WithLabelValues(key.Kind, string(outcome)).Inc()
	switch outcome {
	case Succeeded, Failed, Panic:
		m.duration.WithLabelValues(key.Kind, jobOf(key)).Observe(took.Seconds())
	case NotDue, Cancelled:
	}
}

// jobOf returns the job label of key's runs, reload for the job keyed by the kind alone and account
// for an account's job.
func jobOf(key Key) string {
	if key.ID == "" {
		return "reload"
	}
	return "account"
}

// Succeeded implements Observer.
func (m *Metrics) Succeeded(key Key, at time.Time) {
	m.lastSuccess.WithLabelValues(key.Kind, key.ID).Set(seconds(at))
	m.waited.WithLabelValues(key.Kind, key.ID).Set(0)
}

// BackingOff implements Observer.
func (m *Metrics) BackingOff(key Key, delay time.Duration) {
	m.backoff.WithLabelValues(key.Kind, key.ID).Set(delay.Seconds())
}

// Removed implements Observer. A removed job's own series go with it.
func (m *Metrics) Removed(key Key) {
	m.backoff.DeleteLabelValues(key.Kind, key.ID)
	m.lastSuccess.DeleteLabelValues(key.Kind, key.ID)
	m.late.DeleteLabelValues(key.Kind, key.ID)
	m.waited.DeleteLabelValues(key.Kind, key.ID)
	m.waiting.DeleteLabelValues(key.Kind, key.ID)
}

// seconds returns t in seconds since the epoch, to the millisecond.
func seconds(t time.Time) float64 {
	return float64(t.UnixMilli()) / 1000
}
