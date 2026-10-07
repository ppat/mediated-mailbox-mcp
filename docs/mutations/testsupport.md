# Mutations: testsupport

The demonstrations of the controls whose patches sit in `testsupport/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## -update is off by default, so an ordinary run of go test never writes a golden file by itself

- **Date · evidence:** 2026-09-28 · [pull request #182](https://github.com/ppat/mediated-mailbox-mcp/pull/182)
- **Break:** the flag defaults to true, so an ordinary run enters update mode and writes instead of comparing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/compare`:** `TestGoldenAtMismatchFails`, `TestGoldenAtMissingFileFails`, `TestGoldenMismatchFails`, `TestGoldenMissingFileFails`

## A contract run against a real provider needs a deliberate command of its own, so no accidental or incidental test invocation reaches the provider

- **Date · evidence:** 2026-09-25 · [pull request #158](https://github.com/ppat/mediated-mailbox-mcp/pull/158)
- **Break (1):** the guard lets a live test through under any provider's marker
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail`:** `TestTheLiveContractSkipsUnlessItsCommandStartedIt`, `TestTheLiveContractSkipsUnlessItsCommandStartedIt/another_provider's_marker`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract`:** `TestRequire`, `TestRequire/a_marker_in_another_case`, `TestRequire/another_provider's_marker`
- **Break (2):** the guard lets every run of a live test through
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail`:** `TestTheLiveContractSkipsUnlessItsCommandStartedIt`, `TestTheLiveContractSkipsUnlessItsCommandStartedIt/another_provider's_marker`, `TestTheLiveContractSkipsUnlessItsCommandStartedIt/no_marker`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract`:** `TestRequire`, `TestRequire/a_marker_in_another_case`, `TestRequire/an_empty_marker`, `TestRequire/another_provider's_marker`, `TestRequire/no_marker`
- **Break (3):** the command runs the live test without setting the marker, so it only ever skips
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract`:** `TestInvocation`
- **Break (4):** the guard skips a live test only when its own command started it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/provider/gmail`:** `TestTheLiveContractSkipsUnlessItsCommandStartedIt`, `TestTheLiveContractSkipsUnlessItsCommandStartedIt/another_provider's_marker`, `TestTheLiveContractSkipsUnlessItsCommandStartedIt/no_marker`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract`:** `TestRequire`, `TestRequire/a_marker_in_another_case`, `TestRequire/an_empty_marker`, `TestRequire/another_provider's_marker`, `TestRequire/its_provider's_marker`, `TestRequire/no_marker`

## Golden and GoldenAt fail a test whose content differs from its recorded file

- **Date · evidence:** 2026-09-28 · [pull request #182](https://github.com/ppat/mediated-mailbox-mcp/pull/182)
- **Break:** a differing comparison is computed and discarded, so no mismatch ever fails a test
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/compare`:** `TestGoldenAtMismatchFails`, `TestGoldenMismatchFails`

## Golden and GoldenAt fail a test whose recorded file does not exist, rather than creating it

- **Date · evidence:** 2026-09-28 · [pull request #182](https://github.com/ppat/mediated-mailbox-mcp/pull/182)
- **Break:** a missing recorded file is written from got and the run returns, instead of failing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/compare`:** `TestGoldenAtMissingFileFails`, `TestGoldenMissingFileFails`

## Golden refuses a name that is not a plain relative path inside testdata/golden

- **Date · evidence:** 2026-09-28 · [pull request #182](https://github.com/ppat/mediated-mailbox-mcp/pull/182)
- **Break:** the name is joined onto the golden directory unchecked, so a name holding ".." reaches outside it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/compare`:** `TestGoldenNameMustStayInsideGoldenDir`, `TestGoldenNameMustStayInsideGoldenDir/escape`, `TestGoldenNameMustStayInsideGoldenDir/escape-mid`

## GoldenAt refuses a path that is absolute or not clean

- **Date · evidence:** 2026-09-28 · [pull request #182](https://github.com/ppat/mediated-mailbox-mcp/pull/182)
- **Break (1):** only an unclean path is refused, so an absolute path is followed wherever the machine puts it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/compare`:** `TestGoldenAtPathMustBeRelativeAndClean`, `TestGoldenAtPathMustBeRelativeAndClean/at-absolute`
- **Break (2):** the path is followed unchecked, so an absolute or unclean path reaches the file system
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/compare`:** `TestGoldenAtPathMustBeRelativeAndClean`, `TestGoldenAtPathMustBeRelativeAndClean/at-absolute`, `TestGoldenAtPathMustBeRelativeAndClean/at-unclean`

## No fail file of rapid's own is kept

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** the search for fail files matches nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestRapidFailFilesAreFound`
- **Break (2):** the search for fail files looks under testdata/rapids rather than testdata/rapid
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestRapidFailFilesAreFound`
- **Break (3):** the search for fail files matches every file under any testdata directory
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestNoRapidFailFileIsKept`

## The analyser refuses a failing-case store write from inside a property

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** the store rule also fires on property.Report
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestPlacement`
- **Break (2):** the store rule is removed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestPlacement`

## The analyser refuses a generator report reachable from a property

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** function literals assigned to variables are no longer followed, so a property held in a variable escapes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestPlacement`
- **Break (2):** calls to this package's own functions are no longer followed, so a report call inside a helper escapes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestPlacement`
- **Break (3):** the report rule also fires on property.Check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestPlacement`
- **Break (4):** the report rule is removed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestPlacement`

## The analyser refuses a link to a pure core's symbol

- **Date · evidence:** 2026-09-23 · [pull request #149](https://github.com/ppat/mediated-mailbox-mcp/pull/149)
- **Break (1):** every go:linkname directive is reported, whatever package it names
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (2):** a go:linkname directive naming a pure core's symbol goes unreported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`

## The analyser refuses a read of the environment outside a deployable's composition root

- **Date · evidence:** 2026-09-26 · [pull request #177](https://github.com/ppat/mediated-mailbox-mcp/pull/177)
- **Break (1):** (*exec.Cmd).Environ is not reported, so a command's inherited environment escapes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (2):** os.ExpandEnv is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (3):** only a call is reported, so a reader passed on as a function value escapes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (4):** a deployable's internal packages are exempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (5):** a package outside this module is checked like one inside it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (6):** any function named like a reader of package os is reported, whatever package declares it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (7):** every file of a composition root's package is exempt, not only main.go
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (8):** no read of the environment is reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (9):** the readers of package syscall are not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (10):** test files are no longer exempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`
- **Break (11):** the test tooling is no longer exempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestEnvironment`

## The analyser refuses a write to a pure core's package-level variable

- **Date · evidence:** 2026-09-23 · [pull request #149](https://github.com/ppat/mediated-mailbox-mcp/pull/149)
- **Break (1):** taking a variable's address goes unreported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (2):** a line directive written as a block comment goes unreported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (3):** a line directive naming a test file makes a pure core's variable pass for a test's in its own package
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (4):** every pure-core file holding a comment is reported as carrying a line directive
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (5):** a line directive in a pure core's file goes unreported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (6):** any package may write a variable whose file name, as export data gives it, is a test file's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestAVariableNamedInATestFileIsTheCoresOutsideItsPackage`, `TestAVariableNamedInATestFileIsTheCoresOutsideItsPackage/another_package`
- **Break (7):** a write naming another package's variable through its package goes unreported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (8):** a write to a variable a pure core's test file declares is reported as a write to the pure core
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestAVariableNamedInATestFileIsTheCoresOutsideItsPackage`, `TestAVariableNamedInATestFileIsTheCoresOutsideItsPackage/its_external_tests`, `TestAVariableNamedInATestFileIsTheCoresOutsideItsPackage/the_declaring_package`, `TestGlobals`
- **Break (9):** the write rule reports nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`

## The analyser refuses package-level state declared in a pure core

- **Date · evidence:** 2026-09-23 · [pull request #149](https://github.com/ppat/mediated-mailbox-mcp/pull/149)
- **Break (1):** a variable of any type other than error is admitted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (2):** an error value made by errors.New from a constant message is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (3):** an error value is admitted whatever it holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (4):** an error value is refused like any other variable
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`
- **Break (5):** only the shared pure library counts as a pure core, so a deployable's internal core and a library's core go unchecked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestGlobals`

## The ban-proof script refuses a depguard list that is not strict or carries a deny key

- **Date · evidence:** 2026-09-29 · [pull request #189](https://github.com/ppat/mediated-mailbox-mcp/pull/189)
- **Break (1):** the deny check fires on every list without a deny key and passes one with it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestConfigProblems`
- **Break (2):** the deny check never fires, so a list carrying a deny key passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestConfigProblems`, `TestConfigProblems/import_list_with_a_deny_entry`, `TestConfigProblems/import_list_with_a_deny_key_and_no_value`, `TestConfigProblems/import_list_with_an_empty_deny_key`
- **Break (3):** only a deny key holding entries is refused, so one written empty or with no value passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestConfigProblems`, `TestConfigProblems/import_list_with_a_deny_key_and_no_value`, `TestConfigProblems/import_list_with_an_empty_deny_key`
- **Break (4):** the list-mode check fires on a strict list and passes every other mode
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestConfigProblems`
- **Break (5):** the list-mode check never fires, so a lax list, an original one and one with no mode written pass
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestConfigProblems`, `TestConfigProblems/import_list_in_the_original_mode`, `TestConfigProblems/import_list_with_no_mode_written`, `TestConfigProblems/lax_import_list`
- **Break (6):** a list with no list-mode written passes, though depguard then reads it in the original mode
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestConfigProblems`, `TestConfigProblems/import_list_with_no_mode_written`

## The ban-proof script refuses a file a build that ships compiles that the gating lint does not read

- **Date · evidence:** 2026-09-30 · [pull request #198](https://github.com/ppat/mediated-mailbox-mcp/pull/198)
- **Break (1):** a finding sits on the file's first line rather than its package clause, where no want can sit after a build constraint
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (2):** the lint's listing sets none of the configuration's tags, so a file built only without one is taken as read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (3):** the lint's listing sets CGO_ENABLED=0 as the shipped runs do, so a file built only without cgo is taken as read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (4):** no file is ever refused as unread
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (5):** the tagged run is compared with the configuration's tags alone, so a violation file the lint run with its tag reads is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (6):** the files of packages ./... does not list are compared too, so a testdata package's file is refused a second time, where no want can sit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (7):** every run's files are compared with the lint run with the banproof tag, so a file built only without that tag, which the gating lint reads, is refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`

## The ban-proof script refuses a go vet step whose build tags differ from the lint configuration's

- **Date · evidence:** 2026-09-30 · [pull request #198](https://github.com/ppat/mediated-mailbox-mcp/pull/198)
- **Break (1):** the comparison of the go vet step's tags with the configuration's is skipped, so any tags pass
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestVetTagProblems`, `TestVetTagProblems/a_tag_the_configuration_does_not_set`, `TestVetTagProblems/a_tag_the_configuration_sets_left_out`, `TestVetTagProblems/no_tags`, `TestVetTagProblems/tags_given_twice,_the_first_matching`, `TestVetTagProblemsNamesBothLists`
- **Break (2):** the first -tags on the go vet line is read rather than the last, which the go command keeps, so a step whose later -tags differ passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestVetTagProblems`, `TestVetTagProblems/tags_given_twice,_the_first_matching`, `TestVetTagProblems/tags_given_twice,_the_last_matching`
- **Break (3):** the go vet step's tags are compared with a fixed list rather than the configuration's, so the step passes only while that list happens to match
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestVetTagProblems`, `TestVetTagProblems/same_tags`, `TestVetTagProblems/same_tags_in_another_order,_joined_by_=`, `TestVetTagProblems/tags_given_twice,_the_last_matching`
- **Break (4):** only a workflow with more than one go vet step is refused, so one with none is read as if it had one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestVetTagProblems`, `TestVetTagProblems/no_go_vet_step`
- **Break (5):** a go vet step setting only some of the configuration's tags passes, so a file only a tag it leaves out selects is linted and never vetted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestVetTagProblems`, `TestVetTagProblems/a_tag_the_configuration_sets_left_out`
- **Break (6):** only a workflow with no go vet step is refused, so a second step running the analysers with other tags passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestVetTagProblems`, `TestVetTagProblems/two_go_vet_steps`

## The ban-proof script refuses a package the build of ./... reaches that ./... does not list

- **Date · evidence:** 2026-09-30 · [pull request #198](https://github.com/ppat/mediated-mailbox-mcp/pull/198)
- **Break (1):** every package in the module cache passes, so a nested module published under this module's path is not refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (2):** a package's cgo files are not read, so an import in one is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (3):** CGO_ENABLED=0 is not set, so a file built only without cgo is not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (4):** the darwin/amd64 configuration is not run, so a file only it builds is not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (5):** the darwin/arm64 configuration is not run, so a file only it builds is not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (6):** another module's package passes wherever its directory sits, so a module go.work adds or a local replacement passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`, `TestUnlintedRunsEveryShippedConfiguration`
- **Break (7):** a finding's line is read from the position a line directive gives, so it moves off the import
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (8):** the linux/amd64 configuration is not run, so a file only it builds is not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (9):** the linux/arm64 configuration is not run, so a file only it builds is not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (10):** a package go list could not load is judged like any other rather than failing the check
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindingsCleanAndBroken`
- **Break (11):** the module cache is taken to be the filesystem root, so a package of a module go.work adds passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (12):** a module replaced by another version is taken for one replaced by a directory, so it is reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (13):** a module replaced by a directory under the module cache passes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (14):** no package is ever refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`, `TestUnlintedFindingsCleanAndBroken`, `TestUnlintedRunsEveryShippedConfiguration`
- **Break (15):** the standard library is not exempt, so an import of it is reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (16):** no run sets the banproof tag, so a violation file's imports are not refused
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`
- **Break (17):** an import in a package ./... does not list is reported too, where no want can sit
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (18):** a refused package only packages from the module cache import is not reported, since no file of this module imports it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (19):** a refused package that another refused package imports counts as reached through the module cache, so it is reported at go.mod as well
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedFindings`
- **Break (20):** the configurations run only with the banproof tag, so a file built only without it is not read
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof`:** `TestUnlintedRunsEveryShippedConfiguration`

## The chain applies from an empty database on every test run

- **Date · evidence:** 2026-09-24 · [pull request #153](https://github.com/ppat/mediated-mailbox-mcp/pull/153)
- **Break (1):** goose applies the chain to the database the migration URL names rather than the one just created empty
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/postgres`:** `TestTheChainFailsFromEmptyOnAMigrationOnlyTheCurrentShapeAccepts`, `TestTheChainFailsFromEmptyOnAMigrationOnlyTheCurrentShapeAccepts/from_empty`
- **Break (2):** a migration that fails does not fail the chain's application
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/postgres`:** `TestTheChainFailsFromEmptyOnAMigrationOnlyTheCurrentShapeAccepts`, `TestTheChainFailsFromEmptyOnAMigrationOnlyTheCurrentShapeAccepts/from_empty`

## The crash harness checks persistence after every crash, and replays the reduced sequence against PostgreSQL

- **Date · evidence:** 2026-09-30 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196)
- **Break (1):** neither draw places the crash among the target's operations
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestBothDrawsPlaceTheCrash`, `TestBothDrawsPlaceTheCrash/rapid-draw`, `TestBothDrawsPlaceTheCrash/sampled-mix`, `TestTheHarnessFindsAPlantedFault`
- **Break (2):** the cleanup diagnoses the last case drawn whenever the subtest failed, a replay's failure included, and reports the model as differing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestTheReplaysFindAFaultOnlyPostgreSQLHas`
- **Break (3):** the sequences drawn from fixed seeds are replayed against the model instead of PostgreSQL
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestTheReplaysFindAFaultOnlyPostgreSQLHas`
- **Break (4):** no sequence drawn from a fixed seed is replayed against PostgreSQL
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestTheReplaysFindAFaultOnlyPostgreSQLHas`
- **Break (5):** the harness runs recovery after a crash and never checks persistence
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestRunRecoversAndChecksAfterEachCrash`, `TestTheHarnessFindsAPlantedFault`
- **Break (6):** a crash runs no recovery before persistence is checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestRunRecoversAndChecksAfterEachCrash`
- **Break (7):** a sequence that fails against the model is never replayed against PostgreSQL
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestTheHarnessFindsAPlantedFault`
- **Break (8):** a replay's world is built from the setup of the last case the search drew rather than the replayed case's own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestTheReplaysFindAFaultOnlyPostgreSQLHas`
- **Break (9):** every replay against PostgreSQL replays the sequence of the same seed
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestTheReplaysFindAFaultOnlyPostgreSQLHas`
- **Break (10):** only one sequence drawn from a fixed seed is replayed against PostgreSQL, whatever the number configured
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/crash`:** `TestTheReplaysFindAFaultOnlyPostgreSQLHas`

## The failing-case store keeps a found failure through a generator edit

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** the store keeps the first generated case rather than the reduced failing one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestAStoredFailingCaseOutlivesAGeneratorEdit`
- **Break (2):** stored cases replay whatever shape they were written with
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestAStoredFailingCaseOutlivesAGeneratorEdit`
- **Break (3):** the cleanup stores the last case whenever the test failed, without replaying it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestAnUnrelatedFailureStoresNothing`
- **Break (4):** the cleanup never writes the store
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestAStoredFailingCaseOutlivesAGeneratorEdit`

## The gating run requires a non-zero seed

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** the seed check fires on a set, non-zero seed and passes an unset or zero one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestARunWithoutTheGatingSettingsFails`, `TestAStoredFailingCaseOutlivesAGeneratorEdit`, `TestAnUnrelatedFailureStoresNothing`, `TestTheReportFailsAGeneratorThatMissesItsMix`
- **Break (2):** the seed check never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestARunWithoutTheGatingSettingsFails`, `TestARunWithoutTheGatingSettingsFails/report/seed_unset`, `TestARunWithoutTheGatingSettingsFails/report/seed_zero`, `TestARunWithoutTheGatingSettingsFails/store/seed_unset`, `TestARunWithoutTheGatingSettingsFails/store/seed_zero`

## The gating run requires rapid's fail file switched off

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** the fail-file check fires when rapid's fail file is switched off and passes when it is on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestARunWithoutTheGatingSettingsFails`, `TestAStoredFailingCaseOutlivesAGeneratorEdit`, `TestAnUnrelatedFailureStoresNothing`, `TestTheReportFailsAGeneratorThatMissesItsMix`
- **Break (2):** the fail-file check never fires
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestARunWithoutTheGatingSettingsFails`, `TestARunWithoutTheGatingSettingsFails/report/no_fail_file_false`, `TestARunWithoutTheGatingSettingsFails/report/no_fail_file_unset`, `TestARunWithoutTheGatingSettingsFails/store/no_fail_file_false`, `TestARunWithoutTheGatingSettingsFails/store/no_fail_file_unset`

## The generator report fails a generator that misses its stated mix

- **Date · evidence:** 2026-09-22 · [pull request #142](https://github.com/ppat/mediated-mailbox-mcp/pull/142)
- **Break (1):** a kind is reported short when it reaches its minimum and passed when it misses it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/core/sensitivity`:** `TestNoBodyCarriesADenyingSensitivityMix`
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestARunWithoutTheGatingSettingsFails`, `TestTheReportFailsAGeneratorThatMissesItsMix`
- **Break (2):** no kind is ever below its minimum
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestTheReportFailsAGeneratorThatMissesItsMix`

## The operation sampler draws each operation under a fresh set of weights from rapid's own stream

- **Date · evidence:** 2026-09-29 · [pull request #196](https://github.com/ppat/mediated-mailbox-mcp/pull/196)
- **Break (1):** every sequence draws under the same weights, whatever rapid's stream holds
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestTheWeightsComeFromRapidsStream`
- **Break (2):** each operation is drawn with the same chance, whatever its weight
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestTheSamplerDrawsByWeight`
- **Break (3):** with every weight zero the first name is always drawn
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/property`:** `TestAllZeroWeightsWeighTheSame`

## The raw SQL analyser refuses a statement the UI runs other than through the data-access library

- **Date · evidence:** 2026-10-02 · [pull request #256](https://github.com/ppat/mediated-mailbox-mcp/pull/256)
- **Break (1):** parameters are compared by the names they are printed with, so a parameter spelled through an alias escapes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (2):** a batch sent or rows copied in are not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (3):** only the driver's own methods are reported, so a generated subsection's handle, an interface embedding it and a wrapper escape
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (4):** the lower-level connection's own methods are matched by their full names, so an interface or wrapper the UI declares with their names and parameters escapes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (5):** the lower-level connection's ExecParams, ExecBatch, CopyTo, its CopyFrom and StartPipeline are not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (6):** every method named like a statement runner is reported, whatever its parameters
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (7):** a statement prepared on the driver's connection or transaction is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (8):** no statement is reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (9):** every package of the module is checked, the data-access library included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`
- **Break (10):** test files are no longer exempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRawSQL`

## The routes analyser refuses a route the UI registers on a mux other than through its recording mux

- **Date · evidence:** 2026-09-30 · [pull request #199](https://github.com/ppat/mediated-mailbox-mcp/pull/199)
- **Break (1):** an interface method's parameters are compared by the names they are printed with, so a parameter spelled through an alias escapes
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (2):** a concrete type's method named and typed like the mux's is reported, the recording mux's own Handle included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (3):** a registration on the default mux through http.Handle or http.HandleFunc is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (4):** every method named Handle is exempt, whatever type or package declares it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (5):** every function of the recording mux's package is exempt, not only its Handle method
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (6):** no use of the four functions is reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (7):** a registration through an interface's or a type parameter's Handle or HandleFunc is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (8):** every method named Handle or HandleFunc on an interface is reported, whatever its parameters
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (9):** every package of the module is checked, the mediator included
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`
- **Break (10):** test files are no longer exempt
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestRoutes`

## The txhelper analyser holds every generated data-access function to the transaction helper, except the accounts listing, the read of oauth_clients, delta sync's re-seal of a client secret and the UI's client setup

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** taking the transaction parameter's address is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (2):** taking the address of any variable is reported, not only of the transaction parameter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (3):** an assignment to the transaction parameter is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (4):** an assignment to any variable is reported, not only to the transaction parameter
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (5):** the exception covers every statement of the three exempt subsections, not only the listing, the client read and the client secret's re-seal
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (6):** every use of the three exempt subsections' New is accepted, chained or not
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (7):** a New called inside a literal passed to tx.Run is accepted whatever handle it is given
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (8):** a New called on the innermost literal's own transaction is reported as the enclosing literal's
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (9):** every package under db is a subsection, whatever its New returns
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (10):** a declared function passed to tx.Run is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (11):** a New called on an enclosing literal's transaction inside a nested literal is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (12):** only a package directly below db is a subsection, so a nested one is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (13):** a New called outside every literal passed to tx.Run is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (14):** a package outside db whose New returns its Queries is taken as a subsection
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (15):** a range clause assigning the transaction parameter is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (16):** delta sync's re-seal of a client secret is no longer exempt, so its write outside an account's transaction is reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (17):** the analyser reports nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (18):** the UI's client setup statements lose their exception
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (19):** subsections are recognised by a list of names, so a subsection the list leaves out is not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (20):** test files are not checked
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (21):** New used as a value is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (22):** WithTx is not reported
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`

## The txhelper analyser holds the base policy's subsection and tx.RunBase to each other

- **Date · evidence:** 2026-10-03 · [pull request #258](https://github.com/ppat/mediated-mailbox-mcp/pull/258)
- **Break (1):** the base policy's subsection built in an account's transaction is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (2):** another subsection built in a base-policy transaction is accepted
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
- **Break (3):** a literal passed to tx.RunBase is not a transaction literal, so every subsection built in one is reported as outside the helper
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/testsupport/analysis`:** `TestTxHelper`
