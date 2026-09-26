package settings_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

type config struct {
	Scanner scanner `yaml:"scanner"`
	Server  server  `yaml:"server"`
}

type scanner struct {
	Triggers  map[string][]string `yaml:"triggers"`
	LinkWords []string            `yaml:"link_words"`
	Threshold float64             `yaml:"threshold"`
}

type server struct {
	Listen  string           `yaml:"listen" settings:"required"`
	Timeout time.Duration    `yaml:"timeout"`
	Workers int              `yaml:"workers"`
	Debug   bool             `yaml:"debug"`
	Banner  string           `yaml:"banner"`
	Routes  map[string]route `yaml:"routes"`
}

type route struct {
	Target string `yaml:"target" settings:"required"`
	Weight int    `yaml:"weight"`
}

func defaults() config {
	return config{
		Scanner: scanner{
			Triggers:  map[string][]string{"de": {"Code"}, "en": {"code"}},
			LinkWords: []string{"verify"},
			Threshold: 0.5,
		},
		Server: server{Timeout: 5 * time.Second, Workers: 2, Banner: "yes: no"},
	}
}

// listen sets the one required value, so a load that is not about it succeeds.
const listen = "MEDIATED_MAILBOX_SERVER__LISTEN=:8080"

// load writes file, when it is not empty, names it by the flag, and loads. A path to the file in an
// error is written as FILE.
func load(t *testing.T, file string, args []string, env ...string) (settings.Loaded[config], error) {
	t.Helper()
	path := writeFile(t, file)
	if path != "" {
		args = append([]string{"--config-file=" + path}, args...)
	}
	loaded, err := settings.Load(defaults(), args, env)
	if err != nil && path != "" {
		err = errors.New(strings.ReplaceAll(err.Error(), path, "FILE"))
	}
	return loaded, err
}

func writeFile(t *testing.T, text string) string {
	t.Helper()
	if text == "" {
		return ""
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustLoad(t *testing.T, file string, args []string, env ...string) settings.Loaded[config] {
	t.Helper()
	loaded, err := load(t, file, args, env...)
	if err != nil {
		t.Fatalf("Load refused: %v", err)
	}
	return loaded
}

func TestLayersApplyPerValueInOrder(t *testing.T) {
	file := `
scanner:
  triggers:
    de: [Datei]
  link_words: [confirm, sign in]
server:
  workers: 4
`
	loaded := mustLoad(t, file,
		[]string{"--scanner.triggers.de=[Flagge]", "--server.debug=true"},
		listen,
		"MEDIATED_MAILBOX_SCANNER__TRIGGERS__DE=[Umgebung]",
		"MEDIATED_MAILBOX_SCANNER__TRIGGERS__FR=[code, 'mot de passe']",
		"MEDIATED_MAILBOX_SCANNER__LINK_WORDS=[log in]",
		"MEDIATED_MAILBOX_SERVER__WORKERS=6",
		"MEDIATED_MAILBOX_SERVER__TIMEOUT=90s",
		"PATH=/usr/bin",
	)
	want := config{
		Scanner: scanner{
			Triggers: map[string][]string{
				"de": {"Flagge"},
				"en": {"code"},
				"fr": {"code", "mot de passe"},
			},
			LinkWords: []string{"log in"},
			Threshold: 0.5,
		},
		Server: server{Listen: ":8080", Timeout: 90 * time.Second, Workers: 6, Debug: true, Banner: "yes: no"},
	}
	if diff := cmp.Diff(want, loaded.Config, compare.Options); diff != "" {
		t.Errorf("configuration (-want +got):\n%s", diff)
	}
}

func TestEachValueRecordsTheLayerThatSetIt(t *testing.T) {
	file := `
scanner:
  triggers:
    de: [Datei]
  link_words: [confirm]
server:
  routes:
    api:
      target: http://api
`
	loaded := mustLoad(t, file,
		[]string{"--scanner.triggers.de=[Flagge]", "--server.routes.api.weight=3"},
		listen,
		"MEDIATED_MAILBOX_SCANNER__TRIGGERS__DE=[Umgebung]",
		"MEDIATED_MAILBOX_SCANNER__TRIGGERS__FR=[code]",
	)
	for i := range loaded.Values {
		if loaded.Values[i].Source.Layer == settings.File {
			loaded.Values[i].Source.Name = strings.Replace(loaded.Values[i].Source.Name, filepath.Dir(loaded.Values[i].Source.Name), "DIR", 1)
		}
	}
	want := []settings.Value{
		{Path: "scanner.link_words", Source: settings.Source{Layer: settings.File, Name: "DIR/config.yaml line 5"}, Value: []string{"confirm"}},
		{Path: "scanner.threshold", Source: settings.Source{Layer: settings.Default}, Value: 0.5},
		{Path: "scanner.triggers.de", Source: settings.Source{Layer: settings.Flag, Name: "--scanner.triggers.de"}, Value: []string{"Flagge"}},
		{Path: "scanner.triggers.en", Source: settings.Source{Layer: settings.Default}, Value: []string{"code"}},
		{Path: "scanner.triggers.fr", Source: settings.Source{Layer: settings.Environment, Name: "MEDIATED_MAILBOX_SCANNER__TRIGGERS__FR"}, Value: []string{"code"}},
		{Path: "server.banner", Source: settings.Source{Layer: settings.Default}, Value: "yes: no"},
		{Path: "server.debug", Source: settings.Source{Layer: settings.Default}, Value: false},
		{Path: "server.listen", Source: settings.Source{Layer: settings.Environment, Name: "MEDIATED_MAILBOX_SERVER__LISTEN"}, Value: ":8080"},
		{Path: "server.routes.api.target", Source: settings.Source{Layer: settings.File, Name: "DIR/config.yaml line 9"}, Value: "http://api"},
		{Path: "server.routes.api.weight", Source: settings.Source{Layer: settings.Flag, Name: "--server.routes.api.weight"}, Value: 3},
		{Path: "server.timeout", Source: settings.Source{Layer: settings.Default}, Value: 5 * time.Second},
		{Path: "server.workers", Source: settings.Source{Layer: settings.Default}, Value: 2},
	}
	if diff := cmp.Diff(want, loaded.Values, compare.Options); diff != "" {
		t.Errorf("values (-want +got):\n%s", diff)
	}
}

func TestAListFromALaterLayerReplacesTheWholeList(t *testing.T) {
	loaded := mustLoad(t, "scanner:\n  link_words: [confirm, reset]\n", nil, listen, "MEDIATED_MAILBOX_SCANNER__LINK_WORDS=[log in]")
	if diff := cmp.Diff([]string{"log in"}, loaded.Config.Scanner.LinkWords, compare.Options); diff != "" {
		t.Errorf("link words (-want +got):\n%s", diff)
	}
}

func TestTheFileIsReadOnlyWhenNamed(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"config.yaml", "config.yml", "settings.yaml", "mediated-mailbox.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("server:\n  workers: 9\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	t.Run("no file is named", func(t *testing.T) {
		if got := mustLoad(t, "", nil, listen).Config.Server.Workers; got != 2 {
			t.Errorf("workers %d, want the default 2", got)
		}
	})
	t.Run("the environment names the file", func(t *testing.T) {
		path := writeFile(t, "server:\n  workers: 7\n")
		if got := mustLoad(t, "", nil, listen, "MEDIATED_MAILBOX_CONFIG_FILE="+path).Config.Server.Workers; got != 7 {
			t.Errorf("workers %d, want the file's 7", got)
		}
	})
	t.Run("the flag's file wins over the environment's", func(t *testing.T) {
		envPath := writeFile(t, "server:\n  workers: 7\n")
		if got := mustLoad(t, "server:\n  workers: 8\n", nil, listen, "MEDIATED_MAILBOX_CONFIG_FILE="+envPath).Config.Server.Workers; got != 8 {
			t.Errorf("workers %d, want the flag's file's 8", got)
		}
	})
}

// Two loads in one process, from one defaults value, hold nothing of each other or of the defaults.
func TestTwoConfigurationsShareNothing(t *testing.T) {
	d := defaults()
	first, err := settings.Load(d, []string{"--scanner.triggers.fr=[code]"}, []string{listen, "MEDIATED_MAILBOX_SERVER__ROUTES__API__TARGET=http://api"})
	if err != nil {
		t.Fatal(err)
	}
	first.Config.Scanner.Triggers["en"][0] = "changed"
	first.Config.Scanner.LinkWords[0] = "changed"
	second, err := settings.Load(d, nil, []string{"MEDIATED_MAILBOX_SERVER__LISTEN=:9090"})
	if err != nil {
		t.Fatal(err)
	}
	want := defaults()
	want.Server.Listen = ":9090"
	if diff := cmp.Diff(want, second.Config, compare.Options); diff != "" {
		t.Errorf("the second configuration (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(defaults(), d, compare.Options); diff != "" {
		t.Errorf("the defaults after two loads (-want +got):\n%s", diff)
	}
}

func TestAValueFromTheEnvironmentOrAFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  []string
		want config
	}{
		{
			name: "a string takes the text verbatim",
			args: []string{"--server.listen=[::1]:80 # not a comment"},
			want: withServer(func(s *server) { s.Listen = "[::1]:80 # not a comment" }),
		},
		{
			name: "a string takes a text that reads as a null in YAML",
			env:  []string{"MEDIATED_MAILBOX_SERVER__LISTEN=~"},
			want: withServer(func(s *server) { s.Listen = "~" }),
		},
		{
			name: "an empty string is a string",
			args: []string{"--server.listen="},
			want: withServer(func(s *server) { s.Listen = "" }),
		},
		{
			name: "a list is decoded as YAML",
			args: []string{"--server.listen=x", `--scanner.link_words=[verify, "log in"]`},
			want: withScanner(func(s *scanner) { s.LinkWords = []string{"verify", "log in"} }),
		},
		{
			name: "a map key holds single underscores",
			args: []string{"--server.listen=x", "--scanner.triggers.pt_br=[código]"},
			env:  []string{"MEDIATED_MAILBOX_SCANNER__TRIGGERS__ZH_HANT=[代碼]"},
			want: withScanner(func(s *scanner) { s.Triggers["pt_br"], s.Triggers["zh_hant"] = []string{"código"}, []string{"代碼"} }),
		},
		{
			name: "the language code no is a key and a word, never a boolean",
			args: []string{"--server.listen=x", "--scanner.triggers.no=[no, kode]"},
			want: withScanner(func(s *scanner) { s.Triggers["no"] = []string{"no", "kode"} }),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loaded := mustLoad(t, "", c.args, c.env...)
			if diff := cmp.Diff(c.want, loaded.Config, compare.Options); diff != "" {
				t.Errorf("configuration (-want +got):\n%s", diff)
			}
		})
	}
}

func withServer(edit func(*server)) config {
	c := defaults()
	edit(&c.Server)
	return c
}

func withScanner(edit func(*scanner)) config {
	c := defaults()
	c.Server.Listen = "x"
	edit(&c.Scanner)
	return c
}

func TestAFileValueThatIsNotANull(t *testing.T) {
	loaded := mustLoad(t, "server:\n  listen: \"\"\nscanner:\n  triggers:\n    en: ['null', \"~\"]\n", nil)
	want := withScanner(func(s *scanner) { s.Triggers["en"] = []string{"null", "~"} })
	want.Server.Listen = ""
	if diff := cmp.Diff(want, loaded.Config, compare.Options); diff != "" {
		t.Errorf("configuration (-want +got):\n%s", diff)
	}
}

// Every refusal names the file and line, the environment variable or the flag.
func TestEveryMistakeIsRefusedAtStart(t *testing.T) {
	cases := []struct {
		name string
		file string
		args []string
		env  []string
		want string
	}{
		// The file.
		{
			name: "an unknown key",
			file: "server:\n  listen: x\n  wrokers: 3\n",
			want: "config file FILE: yaml: unmarshal errors:\n  line 3: field wrokers not found in type settings_test.server",
		},
		{
			name: "a case-changed key",
			file: "server:\n  listen: x\n  Workers: 3\n",
			want: "config file FILE: yaml: unmarshal errors:\n  line 3: field Workers not found in type settings_test.server",
		},
		{
			name: "a key given twice",
			file: "server:\n  listen: x\n  workers: 3\n  workers: 4\n",
			want: `config file FILE: yaml: unmarshal errors:
  line 4: mapping key "workers" already defined at line 3`,
		},
		{
			name: "a second document",
			file: "server:\n  listen: x\n---\nserver:\n  workers: 3\n",
			want: "config file FILE: line 3: a second document starts, and only one is read",
		},
		{
			name: "an empty second document",
			file: "server:\n  listen: x\n---\n",
			want: "config file FILE: line 3: a second document starts, and only one is read",
		},
		{
			name: "a file of comments alone",
			file: "# server:\n#   listen: x\n",
			want: "config file FILE: holds no document, and an empty document is a null",
		},
		{
			name: "a null document",
			file: "~\n",
			want: "config file FILE: line 1: an explicit null, and a value is given or left out",
		},
		{
			name: "a key with no value",
			file: "server:\n  listen:\n",
			want: "config file FILE: line 2: an explicit null, and a value is given or left out",
		},
		{
			name: "a null section",
			file: "server: null\n",
			want: "config file FILE: line 1: an explicit null, and a value is given or left out",
		},
		{
			name: "a null in a list",
			file: "server:\n  listen: x\nscanner:\n  link_words: [verify, ~]\n",
			want: "config file FILE: line 4: an explicit null, and a value is given or left out",
		},
		{
			name: "an alias",
			file: "server:\n  listen: &l x\nscanner:\n  triggers:\n    en: [*l]\n",
			want: "config file FILE: line 5: an alias, and aliases are refused",
		},
		{
			name: "a merge key with an alias",
			file: "base: &b\n  workers: 3\nserver:\n  <<: *b\n  listen: x\n",
			want: "config file FILE: line 4: a merge key, and merge keys are refused",
		},
		{
			name: "a merge key without an alias",
			file: "server:\n  <<: {workers: 3}\n  listen: x\n",
			want: "config file FILE: line 2: a merge key, and merge keys are refused",
		},
		{
			name: "a type error",
			file: "server:\n  listen: x\n  workers: many\n",
			want: "config file FILE: yaml: unmarshal errors:\n  line 3: cannot unmarshal !!str `many` into int",
		},
		{
			name: "a YAML word for true in the file",
			file: "server:\n  listen: x\n  debug: yes\n",
			want: `config file FILE: line 3: "yes" is not a boolean, and a boolean is true or false`,
		},
		{
			name: "a YAML word for false in the file",
			file: "server:\n  listen: x\n  debug: off\n",
			want: `config file FILE: line 3: "off" is not a boolean, and a boolean is true or false`,
		},
		{
			name: "a file that cannot be read",
			args: []string{"--config-file=/nonexistent/config.yaml"},
			env:  []string{listen},
			want: "config file /nonexistent/config.yaml: open /nonexistent/config.yaml: no such file or directory",
		},
		{
			name: "a map key with a hyphen",
			file: "server:\n  listen: x\nscanner:\n  triggers:\n    pt-BR: [código]\n",
			want: `config file FILE: line 5: the key "pt-BR" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "a map key in upper case",
			file: "server:\n  listen: x\nscanner:\n  triggers:\n    DE: [Code]\n",
			want: `config file FILE: line 5: the key "DE" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "a map key with a double underscore",
			file: "server:\n  listen: x\nscanner:\n  triggers:\n    pt__br: [código]\n",
			want: `config file FILE: line 5: the key "pt__br" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "a map key starting with an underscore",
			file: "server:\n  listen: x\nscanner:\n  triggers:\n    _pt: [código]\n",
			want: `config file FILE: line 5: the key "_pt" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "a map key ending with an underscore",
			file: "server:\n  listen: x\nscanner:\n  triggers:\n    pt_: [código]\n",
			want: `config file FILE: line 5: the key "pt_" is not runs of lower-case letters and digits joined by single underscores`,
		},

		// The environment.
		{
			name: "an unknown environment variable under the prefix",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__WROKERS=3"},
			want: `environment variable MEDIATED_MAILBOX_SERVER__WROKERS: it names no configuration value, and server declares no "wrokers"`,
		},
		{
			name: "an environment variable in mixed case",
			env:  []string{listen, "MEDIATED_MAILBOX_server__workers=3"},
			want: "environment variable MEDIATED_MAILBOX_server__workers: its name is not upper case, and names under MEDIATED_MAILBOX_ are",
		},
		{
			name: "an environment variable with the prefix in lower case",
			env:  []string{listen, "mediated_mailbox_SERVER__WORKERS=3"},
			want: "environment variable mediated_mailbox_SERVER__WORKERS: its name is not upper case, and names under MEDIATED_MAILBOX_ are",
		},
		{
			name: "an environment variable set twice",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__WORKERS=3", "MEDIATED_MAILBOX_SERVER__WORKERS=4"},
			want: "environment variable MEDIATED_MAILBOX_SERVER__WORKERS: it is set twice",
		},
		{
			name: "an environment variable naming a section",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER={workers: 3}"},
			want: "environment variable MEDIATED_MAILBOX_SERVER: it names server, a section rather than a value",
		},
		{
			name: "an environment variable naming a part of a value",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__WORKERS__MAX=3"},
			want: "environment variable MEDIATED_MAILBOX_SERVER__WORKERS__MAX: it names no configuration value, and server.workers is a value with no parts",
		},
		{
			name: "an environment variable of the wrong type",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__WORKERS=many"},
			want: "environment variable MEDIATED_MAILBOX_SERVER__WORKERS: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `many` into int",
		},
		{
			name: "an environment variable holding a null",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__WORKERS=null"},
			want: "environment variable MEDIATED_MAILBOX_SERVER__WORKERS: line 1: an explicit null, and a value is given or left out",
		},
		{
			name: "an empty environment variable for a value other than a string",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__WORKERS="},
			want: "environment variable MEDIATED_MAILBOX_SERVER__WORKERS: holds no document, and an empty document is a null",
		},
		{
			name: "an environment variable holding a list with a null",
			env:  []string{listen, "MEDIATED_MAILBOX_SCANNER__LINK_WORDS=[verify, null]"},
			want: "environment variable MEDIATED_MAILBOX_SCANNER__LINK_WORDS: line 1: an explicit null, and a value is given or left out",
		},
		{
			name: "an environment variable holding a second document",
			env:  []string{listen, "MEDIATED_MAILBOX_SCANNER__LINK_WORDS=[verify]\n---\n[reset]"},
			want: "environment variable MEDIATED_MAILBOX_SCANNER__LINK_WORDS: line 2: a second document starts, and only one is read",
		},
		{
			name: "an environment variable with a hyphenated map key",
			env:  []string{listen, "MEDIATED_MAILBOX_SCANNER__TRIGGERS__PT-BR=[código]"},
			want: `environment variable MEDIATED_MAILBOX_SCANNER__TRIGGERS__PT-BR: the key "pt-br" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "an environment variable with a map key starting with an underscore",
			env:  []string{listen, "MEDIATED_MAILBOX_SCANNER__TRIGGERS___PT=[código]"},
			want: `environment variable MEDIATED_MAILBOX_SCANNER__TRIGGERS___PT: the key "_pt" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "an environment variable naming an empty file",
			env:  []string{listen, "MEDIATED_MAILBOX_CONFIG_FILE="},
			want: "environment variable MEDIATED_MAILBOX_CONFIG_FILE: it names no file",
		},

		// The flags.
		{
			name: "an unknown flag",
			args: []string{"--server.wrokers=3"},
			env:  []string{listen},
			want: `flag --server.wrokers: it names no configuration value, and server declares no "wrokers"`,
		},
		{
			name: "a case-changed flag",
			args: []string{"--server.Workers=3"},
			env:  []string{listen},
			want: `flag --server.Workers: it names no configuration value, and server declares no "Workers"`,
		},
		{
			name: "a flag with no value",
			args: []string{"--server.workers", "3"},
			env:  []string{listen},
			want: "flag --server.workers: it has no value, and flags are written --server.workers=VALUE",
		},
		{
			name: "a flag with no value, last",
			args: []string{"--server.debug"},
			env:  []string{listen},
			want: "flag --server.debug: it has no value, and flags are written --server.debug=VALUE",
		},
		{
			name: "an argument that is not a flag",
			args: []string{"serve"},
			env:  []string{listen},
			want: `argument "serve": it is not a flag, and flags are written --path=VALUE`,
		},
		{
			name: "a flag with one dash",
			args: []string{"-server.workers=3"},
			env:  []string{listen},
			want: `argument "-server.workers=3": it is not a flag, and flags are written --path=VALUE`,
		},
		{
			name: "a flag given twice",
			args: []string{"--server.workers=3", "--server.workers=4"},
			env:  []string{listen},
			want: "flag --server.workers: it is given twice",
		},
		{
			name: "a flag of the wrong type",
			args: []string{"--server.workers=many"},
			env:  []string{listen},
			want: "flag --server.workers: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `many` into int",
		},
		{
			name: "a flag holding a null",
			args: []string{"--server.workers=~"},
			env:  []string{listen},
			want: "flag --server.workers: line 1: an explicit null, and a value is given or left out",
		},
		{
			name: "a flag holding an alias",
			args: []string{"--scanner.link_words=[&a verify, *a]"},
			env:  []string{listen},
			want: "flag --scanner.link_words: line 1: an alias, and aliases are refused",
		},
		{
			name: "a flag naming a section",
			args: []string{"--scanner.triggers={de: [x]}"},
			env:  []string{listen},
			want: "flag --scanner.triggers: it names scanner.triggers, a section rather than a value",
		},
		{
			name: "a flag with a map key in upper case",
			args: []string{"--scanner.triggers.DE=[Code]"},
			env:  []string{listen},
			want: `flag --scanner.triggers.DE: the key "DE" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "a flag with a map key holding a double underscore",
			args: []string{"--scanner.triggers.pt__br=[código]"},
			env:  []string{listen},
			want: `flag --scanner.triggers.pt__br: the key "pt__br" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "a flag with a map key ending with an underscore",
			args: []string{"--scanner.triggers.pt_=[código]"},
			env:  []string{listen},
			want: `flag --scanner.triggers.pt_: the key "pt_" is not runs of lower-case letters and digits joined by single underscores`,
		},
		{
			name: "a flag naming an empty file",
			args: []string{"--config-file="},
			env:  []string{listen},
			want: "flag --config-file: it names no file",
		},

		{
			name: "a flag holding a YAML word for true",
			args: []string{"--server.debug=yes"},
			env:  []string{listen},
			want: `flag --server.debug: line 1: "yes" is not a boolean, and a boolean is true or false`,
		},
		{
			name: "a flag holding a YAML word for false",
			args: []string{"--server.debug=off"},
			env:  []string{listen},
			want: `flag --server.debug: line 1: "off" is not a boolean, and a boolean is true or false`,
		},
		{
			name: "an environment variable holding a YAML word for true",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__DEBUG=on"},
			want: `environment variable MEDIATED_MAILBOX_SERVER__DEBUG: line 1: "on" is not a boolean, and a boolean is true or false`,
		},
		{
			name: "an environment variable holding a YAML word for false",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__DEBUG=no"},
			want: `environment variable MEDIATED_MAILBOX_SERVER__DEBUG: line 1: "no" is not a boolean, and a boolean is true or false`,
		},

		// Required values.
		{
			name: "a required value no layer sets",
			env:  []string{"MEDIATED_MAILBOX_SERVER__WORKERS=3"},
			want: "server.listen is required, and neither the file, MEDIATED_MAILBOX_SERVER__LISTEN nor --server.listen sets it",
		},
		{
			name: "a required value of an empty keyed entry in the file",
			file: "server:\n  listen: x\n  routes:\n    api: {}\n",
			want: "server.routes.api.target is required, and neither the file, MEDIATED_MAILBOX_SERVER__ROUTES__API__TARGET nor --server.routes.api.target sets it",
		},
		{
			name: "a required value of a keyed entry a flag names",
			args: []string{"--server.routes.api.weight=3"},
			env:  []string{listen},
			want: "server.routes.api.target is required, and neither the file, MEDIATED_MAILBOX_SERVER__ROUTES__API__TARGET nor --server.routes.api.target sets it",
		},
		{
			name: "a required value of a keyed entry no layer sets",
			env:  []string{listen, "MEDIATED_MAILBOX_SERVER__ROUTES__API__WEIGHT=3"},
			want: "server.routes.api.target is required, and neither the file, MEDIATED_MAILBOX_SERVER__ROUTES__API__TARGET nor --server.routes.api.target sets it",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := load(t, c.file, c.args, c.env...)
			if err == nil {
				t.Fatalf("Load accepted it, want %q", c.want)
			}
			if diff := cmp.Diff(c.want, err.Error(), compare.Options); diff != "" {
				t.Errorf("error (-want +got):\n%s", diff)
			}
		})
	}
}

// A required value is met by any layer.
func TestARequiredValueIsMetByAnyLayer(t *testing.T) {
	cases := []struct {
		name string
		file string
		args []string
		env  []string
	}{
		{name: "the file", file: "server:\n  listen: x\n"},
		{name: "the environment", env: []string{"MEDIATED_MAILBOX_SERVER__LISTEN=x"}},
		{name: "a flag", args: []string{"--server.listen=x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mustLoad(t, c.file, c.args, c.env...).Config.Server.Listen; got != "x" {
				t.Errorf("listen %q, want x", got)
			}
		})
	}
}

// A section's revision follows its effective value, whichever layer changed it.
func TestASectionsRevisionFollowsItsEffectiveValue(t *testing.T) {
	base := mustLoad(t, "", nil, listen).Revisions
	cases := []struct {
		name          string
		file          string
		args          []string
		env           []string
		scannerChange bool
		serverChange  bool
	}{
		{name: "nothing changes", env: []string{listen}},
		{name: "the file changes a trigger word", file: "scanner:\n  triggers:\n    de: [Datei]\n", env: []string{listen}, scannerChange: true},
		{name: "the environment adds a language", env: []string{listen, "MEDIATED_MAILBOX_SCANNER__TRIGGERS__FR=[code]"}, scannerChange: true},
		{name: "a flag changes the threshold", args: []string{"--scanner.threshold=0.6"}, env: []string{listen}, scannerChange: true},
		{name: "a flag changes the server", args: []string{"--server.workers=3"}, env: []string{listen}, serverChange: true},
		{name: "the file restates a default", file: "scanner:\n  threshold: 0.5\n", env: []string{listen}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mustLoad(t, c.file, c.args, c.env...).Revisions
			if changed := got["scanner"] != base["scanner"]; changed != c.scannerChange {
				t.Errorf("scanner revision changed %v, want %v", changed, c.scannerChange)
			}
			if changed := got["server"] != base["server"]; changed != c.serverChange {
				t.Errorf("server revision changed %v, want %v", changed, c.serverChange)
			}
			if len(got) != 2 || got["scanner"] == "" || got["server"] == "" {
				t.Errorf("revisions %v, want one for each of scanner and server", got)
			}
		})
	}
}

func TestHelpListsEveryValue(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			_, err := settings.Load(defaults(), []string{"--server.workers=3", arg}, nil)
			var help *settings.HelpRequested
			if !errors.As(err, &help) {
				t.Fatalf("Load returned %v, want the help", err)
			}
			compare.Golden(t, "help.txt", []byte(help.Text))
		})
	}
}

type (
	unexported struct {
		value int `yaml:"value"` //nolint:unused // It exists only for Load to refuse it.
	}
	untagged struct {
		Value int
	}
	optioned struct {
		Value int `yaml:"value,omitempty"`
	}
	inlined struct {
		Inner untagged `yaml:",inline"`
	}
	upperCase struct {
		Value int `yaml:"Value"`
	}
	trailingUnderscore struct {
		Value int `yaml:"value_"`
	}
	twice struct {
		A int `yaml:"value"`
		B int `yaml:"value"`
	}
	pointer struct {
		Value *int `yaml:"value"`
	}
	intKeys struct {
		Values map[int]string `yaml:"values"`
	}
	unknownSettingsTag struct {
		Value int `yaml:"value" settings:"optional"`
	}
	requiredSection struct {
		Section struct {
			Value int `yaml:"value"`
		} `yaml:"section" settings:"required"`
	}
	requiredInList struct {
		Items []struct {
			Value int `yaml:"value" settings:"required"`
		} `yaml:"items"`
	}
	fileName struct {
		ConfigFile string `yaml:"config_file"`
	}
	requiredWithDefault struct {
		Value int `yaml:"value" settings:"required"`
	}
	defaultKey struct {
		Values map[string]int `yaml:"values"`
	}
)

// A declaration the layers cannot name or set is refused before any layer is read.
func TestADeclarationThatCannotBeConfiguredIsRefused(t *testing.T) {
	cases := []struct {
		name string
		load func() error
		want string
	}{
		{"a root that is not a struct", loadOf(map[string]int{}), "the root configuration type map[string]int is not a struct"},
		{"an unexported field", loadOf(unexported{}), "settings_test.unexported: field value is unexported, so no layer can set it"},
		{"a field with no tag", loadOf(untagged{}), `settings_test.untagged: field Value has the yaml tag "", and a name is lower-case letters and digits in words joined by single underscores, with no options`},
		{"a tag with an option", loadOf(optioned{}), `settings_test.optioned: field Value has the yaml tag "value,omitempty", and a name is lower-case letters and digits in words joined by single underscores, with no options`},
		{"an inlined field", loadOf(inlined{}), `settings_test.inlined: field Inner has the yaml tag ",inline", and a name is lower-case letters and digits in words joined by single underscores, with no options`},
		{"a name in upper case", loadOf(upperCase{}), `settings_test.upperCase: field Value has the yaml tag "Value", and a name is lower-case letters and digits in words joined by single underscores, with no options`},
		{"a name ending in an underscore", loadOf(trailingUnderscore{}), `settings_test.trailingUnderscore: field Value has the yaml tag "value_", and a name is lower-case letters and digits in words joined by single underscores, with no options`},
		{"a name declared twice", loadOf(twice{}), "value is declared twice"},
		{"a pointer", loadOf(pointer{}), "value: type *int cannot be configured"},
		{"a map with keys that are not strings", loadOf(intKeys{}), "values: a map's keys must be strings"},
		{"an unknown settings tag", loadOf(unknownSettingsTag{}), `value has the settings tag "optional", and the only one is required`},
		{"a required section", loadOf(requiredSection{}), "section is required, and only a value outside a list can be"},
		{"a required field inside a list", loadOf(requiredInList{}), "items.value is required, and only a value outside a list can be"},
		{"the file's own name", loadOf(fileName{}), "the root configuration type declares config_file, whose environment name is the file's"},
		{"a required value with a default", loadOf(requiredWithDefault{Value: 1}), "value is required and has a default"},
		{"a default key outside the alphabet", loadOf(defaultKey{Values: map[string]int{"pt-BR": 1}}), `the default of values: the key "pt-BR" is not runs of lower-case letters and digits joined by single underscores`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.load()
			if err == nil {
				t.Fatalf("Load accepted it, want %q", c.want)
			}
			if diff := cmp.Diff(c.want, err.Error(), compare.Options); diff != "" {
				t.Errorf("error (-want +got):\n%s", diff)
			}
		})
	}
}

func loadOf[T any](defaults T) func() error {
	return func() error {
		_, err := settings.Load(defaults, nil, nil)
		return err
	}
}

// A boolean from the environment or a flag is true or false.
func TestABooleanIsTrueOrFalse(t *testing.T) {
	for _, text := range []string{"true", "false"} {
		t.Run(text, func(t *testing.T) {
			got := mustLoad(t, "", []string{"--server.debug=" + text}, listen).Config.Server.Debug
			if want := text == "true"; got != want {
				t.Errorf("debug %v, want %v", got, want)
			}
		})
	}
}

// An entry the file names with a value set in it is kept.
func TestAKeyedEntryTheFileNamesIsKept(t *testing.T) {
	loaded := mustLoad(t, "server:\n  listen: x\n  routes:\n    api: {target: http://api}\n", nil)
	if diff := cmp.Diff(map[string]route{"api": {Target: "http://api"}}, loaded.Config.Server.Routes, compare.Options); diff != "" {
		t.Errorf("routes (-want +got):\n%s", diff)
	}
}

type (
	defaultEntries struct {
		Routes map[string]target            `yaml:"routes"`
		Labels map[string]map[string]string `yaml:"labels"`
	}
	target struct {
		Target string `yaml:"target" settings:"required"`
	}
)

// An entry the defaults name with nothing set in it exists in the result, so its required values
// are demanded and an empty map entry is kept.
func TestAnEmptyKeyedEntryOfTheDefaultsIsKept(t *testing.T) {
	t.Run("its required values are demanded", func(t *testing.T) {
		_, err := settings.Load(defaultEntries{Routes: map[string]target{"api": {}}}, nil, nil)
		want := "routes.api.target is required, and neither the file, MEDIATED_MAILBOX_ROUTES__API__TARGET nor --routes.api.target sets it"
		if err == nil || err.Error() != want {
			t.Fatalf("Load returned %v, want %q", err, want)
		}
	})
	t.Run("an empty map entry is kept", func(t *testing.T) {
		loaded, err := settings.Load(defaultEntries{Labels: map[string]map[string]string{"api": {}}}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := loaded.Config.Labels["api"]; !ok || len(loaded.Config.Labels) != 1 {
			t.Errorf("labels %v, want the one entry api", loaded.Config.Labels)
		}
	})
}

type (
	switches struct {
		Switches switchSection `yaml:"switches"`
	}
	switchSection struct {
		List  []bool            `yaml:"list"`
		Items []switchItem      `yaml:"items"`
		Named []map[string]bool `yaml:"named"`
	}
	switchItem struct {
		On bool `yaml:"on"`
	}
)

// A boolean inside a value from any layer is true or false, wherever it sits.
func TestABooleanInsideAValueIsTrueOrFalse(t *testing.T) {
	refused := []struct {
		name      string
		file      string
		args, env []string
		want      string
	}{
		{"in a list, from the file", "switches:\n  list: [true, no]\n", nil, nil, `config file FILE: line 2: "no" is not a boolean, and a boolean is true or false`},
		{"in a section in a list, from the file", "switches:\n  items:\n    - on: on\n", nil, nil, `config file FILE: line 3: "on" is not a boolean, and a boolean is true or false`},
		{"in a map in a list, from the file", "switches:\n  named:\n    - {a: false, b: yes}\n", nil, nil, `config file FILE: line 3: "yes" is not a boolean, and a boolean is true or false`},
		{"in a list, from a flag", "", []string{"--switches.list=[true, yes]"}, nil, `flag --switches.list: line 1: "yes" is not a boolean, and a boolean is true or false`},
		{"in a list, from the environment", "", nil, []string{"MEDIATED_MAILBOX_SWITCHES__LIST=[off]"}, `environment variable MEDIATED_MAILBOX_SWITCHES__LIST: line 1: "off" is not a boolean, and a boolean is true or false`},
		{"in a section in a list", "", []string{"--switches.items=[{on: on}]"}, nil, `flag --switches.items: line 1: "on" is not a boolean, and a boolean is true or false`},
		{"in a map in a list", "", nil, []string{"MEDIATED_MAILBOX_SWITCHES__NAMED=[{a: false, b: no}]"}, `environment variable MEDIATED_MAILBOX_SWITCHES__NAMED: line 1: "no" is not a boolean, and a boolean is true or false`},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			args := c.args
			if path := writeFile(t, c.file); path != "" {
				args = append([]string{"--config-file=" + path}, args...)
			}
			_, err := settings.Load(switches{}, args, c.env)
			if err != nil && c.file != "" {
				err = errors.New(strings.ReplaceAll(err.Error(), args[0][len("--config-file="):], "FILE"))
			}
			if err == nil {
				t.Fatalf("Load accepted it, want %q", c.want)
			}
			if diff := cmp.Diff(c.want, err.Error(), compare.Options); diff != "" {
				t.Errorf("error (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("true and false are read", func(t *testing.T) {
		loaded, err := settings.Load(switches{}, []string{"--switches.list=[true, false]", "--switches.items=[{on: true}]", "--switches.named=[{a: false}]"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := switchSection{List: []bool{true, false}, Items: []switchItem{{On: true}}, Named: []map[string]bool{{"a": false}}}
		if diff := cmp.Diff(want, loaded.Config.Switches, compare.Options); diff != "" {
			t.Errorf("switches (-want +got):\n%s", diff)
		}
	})
}

type helpStrings struct {
	Empty string            `yaml:"empty"`
	Lines string            `yaml:"lines"`
	Plain string            `yaml:"plain"`
	ByKey map[string]string `yaml:"by_key"`
}

// Help shows a string default verbatim, apart from an empty one or one holding a newline, which it
// quotes.
func TestHelpQuotesAnEmptyOrMultiLineStringDefault(t *testing.T) {
	_, err := settings.Load(helpStrings{Lines: "one\ntwo", Plain: "yes: no", ByKey: map[string]string{"a": "", "b": "x\ny"}}, []string{"--help"}, nil)
	var help *settings.HelpRequested
	if !errors.As(err, &help) {
		t.Fatalf("Load returned %v, want the help", err)
	}
	for _, want := range []string{
		"\nempty\n  environment: MEDIATED_MAILBOX_EMPTY\n  flag: --empty=VALUE\n  default: \"\"\n",
		"\nlines\n  environment: MEDIATED_MAILBOX_LINES\n  flag: --lines=VALUE\n  default: \"one\\ntwo\"\n",
		"\nplain\n  environment: MEDIATED_MAILBOX_PLAIN\n  flag: --plain=VALUE\n  default: yes: no\n",
		"\n  by_key.a: \"\"\n  by_key.b: \"x\\ny\"\n",
	} {
		if !strings.Contains(help.Text, want) {
			t.Errorf("help lacks %q:\n%s", want, help.Text)
		}
	}
}
