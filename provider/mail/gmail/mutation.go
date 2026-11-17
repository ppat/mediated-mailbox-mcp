package gmail

import (
	"context"
	"errors"
	"fmt"

	"github.com/ppat/mediated-mailbox-mcp/core/authorize"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// labelChange is what one op does in Gmail's terms, the label identifiers it adds and removes on the
// op's message.
type labelChange struct {
	Add    []string
	Remove []string
}

// none reports whether the change adds and removes nothing, as an op that removes only a label the
// account lacks does.
func (c labelChange) none() bool { return len(c.Add) == 0 && len(c.Remove) == 0 }

// opChange translates a built op, resolving the label paths it names through labels. Only a label
// the op adds must exist (ADR-0010). A path it adds that the account lacks is ErrNotFound, and a
// path it removes that the account lacks is left out of the change, since removing it changes
// nothing.
func opChange(op mail.MutationOp, labels labelTable) (labelChange, error) {
	added := func(path string) ([]string, error) {
		id, ok := labels.idByPath[path]
		if !ok {
			return nil, fmt.Errorf("gmail: label %q: %w", path, mail.ErrNotFound)
		}
		return []string{id}, nil
	}
	removed := func(path string) []string {
		id, ok := labels.idByPath[path]
		if !ok {
			return nil
		}
		return []string{id}
	}
	switch op.Verb() {
	case authorize.Label:
		add, err := added(op.AddLabel())
		return labelChange{Add: add}, err
	case authorize.Unlabel:
		return labelChange{Remove: removed(op.RemoveLabel())}, nil
	case authorize.Move:
		add, err := added(op.AddLabel())
		if err != nil {
			return labelChange{}, err
		}
		return labelChange{Add: add, Remove: removed(op.RemoveLabel())}, nil
	case authorize.Archive:
		return labelChange{Remove: []string{mail.Inbox}}, nil
	case authorize.MarkRead:
		return labelChange{Remove: []string{unreadID}}, nil
	case authorize.Star:
		return labelChange{Add: []string{starredID}}, nil
	case authorize.Trash:
		return labelChange{Add: []string{mail.Trash}, Remove: []string{mail.Inbox}}, nil
	case authorize.Spam:
		return labelChange{Add: []string{mail.Spam}, Remove: []string{mail.Inbox}}, nil
	default:
		return labelChange{}, fmt.Errorf("gmail: verb %s: %w", op.Verb(), mail.ErrInvalid)
	}
}

// stopsBatch reports whether an op's error is about the whole call rather than the op, so every op
// after it fails with the same error and without a call. A throttle, a refused credential and a
// cancelled call are.
func stopsBatch(err error) bool {
	return errors.Is(err, mail.ErrThrottled) || errors.Is(err, mail.ErrAuthentication) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
