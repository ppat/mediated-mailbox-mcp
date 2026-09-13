# 0055. Property-based tests here check stated rules without restating the code, and never check what the policy decides

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[C3](../../../USE_CASES.md#c3--content-based-secrets-caught),
[A3](../../../USE_CASES.md#a3--bulk-change-is-reversible),
[A4](../../../USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything)

## Context

An example-based test states one input and the answer expected for it. A property-based test states
a rule that must hold for every input, and a library generates hundreds of inputs to try to break
it. Generation finds the inputs nobody writes by hand, meaning odd orderings, interactions, and
several individually unlikely conditions arriving at once.

The hard part is not running the test. It is finding a rule worth stating. The obvious rule is "the
answer is correct", but checking that needs something that already knows the correct answer. The
field calls such a thing a **test oracle**, meaning anything that can give you the right answer for
an input without running the code being tested. A second implementation written by someone else is
one. A person writing expected answers down by hand is another.

Most code has no test oracle sitting next to it, and the tempting move is to write one. That move is
the trap this record exists to avoid. If the thing you write works the answer out the same way the
code does, it is the same code twice, and it is wrong in the same places. John Hughes, one of the
two authors of the original QuickCheck property-testing library, puts the warning on a line of its
own, *avoid replicating your code in your tests*. His example is testing a list-reversing function
by writing a second function to predict what the reversed list should be. That predictor "is not
easier to write than `reverse`, it is exactly the same function" ([How to Specify
It!](https://doi.org/10.1007/978-3-030-47147-7_4)).

So the question this record answers is which rules can be checked without the test working out the
answer the way the code does, and which of those are worth writing here.

### The kinds of rule people write

These are the field's own names for them. They are terms from software testing rather than project
vocabulary, so they are defined here and not in [DESIGN.md](../../../DESIGN.md)'s Glossary. Each one
below says what the rule is, gives an example, and says what has been measured about how well it
finds bugs.

**1. Model-based property.** Run the real code and a second, deliberately simple version written for
the test, then check they agree. The simple version can be slow and can ignore every performance
concern, because it is never shipped. Amazon tested a storage engine against a plain description of
what storing and fetching means
([ShardStore](https://jamesbornholt.com/papers/shardstore-sosp21.pdf)).

In one experiment, on one data structure with eight deliberately planted bugs, this kind found bugs
fastest, failing after a mean of 5.8 generated inputs against 77 for postconditions (kind 6 below)
and 56 for metamorphic relations (kind 3 below). Those means count only the cases where a property
found the bug at all, across seven of the eight bugs
([Hughes](https://doi.org/10.1007/978-3-030-47147-7_4)). One experiment on one data structure says
nothing about which kind works best in other code. The simple version does not need to have existed
beforehand. Amazon wrote theirs for the tests.

**2. Differential testing.** The same idea, except the second implementation already exists and
somebody else wrote it for their own reasons. Csmith, which generates C programs and compares what
several different compilers make of them, is the best-known example, and its authors report never
having seen two unrelated compilers produce the same wrong output
([Csmith](https://doi.org/10.1145/1993498.1993532)).

**3. Metamorphic relation.** A rule about how two runs relate to each other, where you do not know
the correct answer to either one. Searching for "red shoes" returns some number of results.
Searching for "red shoes in size 10" must return that number or fewer. You never learn what the
right results are, and the rule still catches real faults. This is the standard name in the
literature and the term to search for.

Relations can also find bugs cheaply. One compiler-testing project removed code a run never executed
from a program, compiled both versions with the same compiler, and required the same output. It
found 147 confirmed bugs in eleven months from roughly 1,500 lines of test code, where Csmith, the
differential tester it was compared with, needed 30,000 to 40,000 ([equivalence modulo
inputs](https://doi.org/10.1145/2594291.2594334)).

Six published relation patterns for interfaces that search and filter scored 95.3% against 317
deliberately broken versions, on four small programs written by students. That is after 101 of 418
broken versions were excluded, 28 because they behaved the same as the original and 73 because they
touched error handling ([Segura et al.](https://doi.org/10.1109/TSE.2017.2764464)). Small
evaluations like that overstate. A comparable set of relations that caught 19 of 24 broken versions
when first evaluated caught 105 of 709 against a larger set of broken versions drawn from different
sources, 14.8% ([Saha and Kanewala](https://arxiv.org/abs/1904.07348)). In another study, testers
who had never used the technique wrote relations, and an average of three to six of them closed at
least 90% of the gap between random testing and a real test oracle, though 21 of the 51 sets of
relations produced did not reach that ([Liu et al.](https://doi.org/10.1109/TSE.2013.46)).

**4. Round-trip property.** A metamorphic relation where one operation undoes another, so doing both
gets the original value back. Encoding then decoding. Applying a change then rolling it back. One of
the stronger kinds in the one large study of real code, and among the most common shapes developers
report writing, 11 of 30 interviewed ([Ravi and Coblenz](https://doi.org/10.1145/3764068),
[Goldstein et al.](https://harrisongoldste.in/papers/icse24-pbt-in-practice.pdf)).

**5. Idempotence.** Doing something twice gives the same result as doing it once. If a rule said
that masking an already-masked subject changes nothing, that would be an idempotence rule. This has
to be tested rather than assumed. Of 24 real text sanitizers checked formally, 19 behaved this way,
so roughly one in five did not, though 17 of the 24 came from one family of similar tools
([BEK](https://www.usenix.org/legacy/events/sec11/tech/full_papers/Hooimeijer.pdf)).

**6. Postcondition.** Something that must be true of one call's output, given its input. After
sorting a list, each item is no larger than the one after it. A useful narrow case is asserting that
a specific failure happens for input that should be refused. That case scored highest of all
categories in the one large study of real code, though such tests were only 0.28% of the code
studied, which its authors flag as too rare to trust the ranking. It also requires the generator to
produce only invalid inputs, so it moves work into generation rather than being free ([Ravi and
Coblenz](https://doi.org/10.1145/3764068)).

**7. Invariant on the returned value.** Something that must be true of every value of a type,
whatever produced it. In the same single experiment as kind 1 it was the weakest kind, missing five
of the eight planted bugs. Its author also notes that such properties would all still pass if every
function returned an empty result ([Hughes](https://doi.org/10.1007/978-3-030-47147-7_4)).

**8. Stateful property.** Generate a sequence of operations rather than a single input, and check a
rule after each step. This kind carries most of the published industrial results, though most of
those come from one consultancy whose clients hired it to build exactly this kind of test. It is
named in almost no guide to choosing properties.

**9. A test asserting only that nothing crashes.** The cheapest thing to write, and the kind
developers rate lowest for confidence, used by 7 of 30 interviewed ([Goldstein et
al.](https://harrisongoldste.in/papers/icse24-pbt-in-practice.pdf)).

**10. A test checking which answer a set of rules produced.** Asking whether this sender was
classified correctly, when the classification rules are the only thing that defines "correctly". No
systematic practice of this kind is known. Four anonymization and redaction tools,
[Presidio](https://github.com/microsoft/presidio),
[scrubadub](https://github.com/LeapBeyond/scrubadub), [Phileas](https://github.com/philterd/phileas)
and [ammonia](https://github.com/rust-ammonia/ammonia), run no property-based testing framework, and
each states its expected answers by hand in its tests.

### Which of them are used here

| Kind | Used here |
| --- | --- |
| 1. Model-based property | Yes, when the simple version can be built without copying the rules it checks |
| 2. Differential testing | Yes, where a second implementation already exists. That is rare here. The provider port is implemented by each adapter and by the fake, and [ADR-0043](./0043-no-mocking.md)'s contract suite already covers them |
| 3. Metamorphic relation | Yes |
| 4. Round-trip property | Yes. [ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)'s op log states this rule for rollback |
| 5. Idempotence | Yes |
| 6. Postcondition | Yes, where a stated rule says what must be refused |
| 7. Invariant on the returned value | Yes, but only alongside a check that the value is not empty |
| 8. Stateful property | Yes. [ADR-0045](./0045-crash-injection-testing.md)'s crash harness uses the same shape with a crash step added, and is a separate kind of test decided there |
| 9. Nothing crashes | No. It states no rule this project holds |
| 10. Checking which answer the rules produced | No. See the Decision |

### Two ways a property-based test passes without proving anything

Both apply whichever kind is used.

**The generator never produced an input that could expose the fault.** The test passes, and it looks
exactly like a test that passed because the code is right. Some libraries let you state what mix of
inputs a run must contain and fail the run when the generator misses it, notably [Haskell's
QuickCheck](https://hackage.haskell.org/package/QuickCheck-2.15.0.1/docs/Test-QuickCheck.html#v:checkCoverage)
and [Java's jqwik](https://jqwik.net/docs/current/user-guide.html). No Go library offers this. That
is a fact about the libraries available rather than about the technique, and
[ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) answers it with a generator report the
project writes itself.

**Code that does nothing, or gives one fixed answer, passes the rule.** Every metamorphic relation
is satisfied by some trivial implementation. Code that never restricts anything passes "restrictions
only accumulate". Code that always returns an empty result passes "adding a filter cannot increase
the number of results". This is measured. Of fifteen deliberately broken versions that a published
relation set failed to catch, seven were broken in a way that made the interface return either
nothing or everything for every input ([Segura et al.](https://doi.org/10.1109/TSE.2017.2764464)).
Those authors discount it, writing that such a bug "should be trivially detected by any sensible
manual test", which is true where a person reads the output and not true where the rule is the only
thing checking a mechanism before release.

The same study measured a second route. The other eight of the fifteen broken versions changed both
runs in the same way, so the comparison still matched. No rule relating two runs of the same code
can catch a fault like that. Catching it takes a check that compares the output with something other
than another run of that code, such as a separate simple version, a second implementation, or an
expected answer written down in the test.

The first route changes how such a rule has to be proven. Deliberately breaking the mechanism the
rule appears to guard usually makes the code do less, and doing less is often what passes the rule,
so the test stays green. The second route is not closed by how the rule is proven, only by one of
the other checks just named.

## Decision

- **A property-based test executes a rule the documents already state.** The outcome contract, the
  design, and [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) say what must hold and what must never
  happen, and a property turns one such statement into a test over generated inputs. Two committed
  examples are that no message-body value carrying restricted sensitivity can be constructed, and
  that no scanner output or log ever contains fixture body text. Nobody searches the code for new
  things to assert. A rule worth testing is worth stating in the documents first.
- **The kinds marked yes above are the ones written here, and each test says which kind it is.**
  Naming the kind is what lets a reviewer check that the test does not work out the answer the way
  the code does.
- **A model-based property is written only when the simple version can be built without copying the
  rules it is checking.** Writing the same rules twice produces two copies that are wrong in the
  same way. Whether a simple version is genuinely separate is argued for each property, not assumed
  from the fact that it is in a different file.
- **No property-based test checks which classification the sender classifier or the scan gate
  returns.** The policy rules are the only definition of the correct answer, so a simple version
  would be a copy of them, and a metamorphic relation would have to state what the policy decides.
  Example-based tests carry that logic. This bars a test from checking what the policy decides. It
  does not bar a test executing a stated safety rule over the same logic, where the rule gives the
  same expected answer for every input, that the violation never happens.
- **An invariant on a returned value is written alongside a check that the value is not empty.** On
  its own it is passed by a mechanism that produced nothing, which is the failure this system most
  needs to catch.
- **Gating runs are bounded and deterministic, and deep random search runs out of band.** A failing
  input, once found, is stored and replayed on every later run.
- **For a metamorphic relation, the mutation demonstration breaks whatever would let the rule pass
  without the code doing its job.** [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md)
  requires tests to go red when a mechanism is removed, and for these rules removing a mechanism
  often leaves the rule passing. For a round trip that means breaking the reverse step, because a
  forward step that does nothing leaves nothing to reverse and the rule still holds. For a rule that
  repeating a step changes nothing, it means changing what the step produces into something the step
  would act on again, not deleting the step. A fault that changes both runs the same way passes any
  such rule. That is a limit of these rules which no choice of break removes.

## Alternatives considered

- **Requiring a test oracle before any property-based test may exist.** The case for it is real and
  survives above, which is that a test working the answer out the way the code does proves nothing.
  It was rejected as a blanket rule because it treats one usable kind as the only one. Metamorphic
  relations check real behaviour while stating no expected answer, and they are the best-evidenced
  way to test an interface that searches or filters.
- **Writing every kind the literature reports.** Rejected in two places. A test asserting only that
  nothing crashes states no rule this project holds. An invariant on a returned value, on its own,
  is passed by a mechanism that produced nothing, which is the failure this design treats as
  unacceptable.
- **Property-based tests that check what the sender classifier or the scan gate decides.** No case
  was tabled for one. Rejected because the policy rules are the only definition of the answer, so
  the test would be a copy of them.
- **Searching the code for properties to write.** No case was tabled for one. Rejected because a
  design built on falsifiable deny-by-default outcomes already states its rules, and a rule worth
  testing is worth stating in the documents first.

## Consequences

- A property bound to a stated rule survives any rewrite that preserves the rule.
- Whether a simple version is genuinely separate from the code it checks is argued for each property
  and is never settled by construction. Knight and Leveson's experiment with separately written
  implementations of one specification reports that they failed together more often than chance
  predicts ([Knight and Leveson](https://doi.org/10.1109/TSE.1986.6312924)), while Csmith's authors
  report the opposite for compilers, so the answer depends on the case.
- The rules these tests execute are stated in other records, so changing one of those rules changes
  what its test asserts. The op log's exact-restore rule
  ([ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)), the released body's form
  ([ADR-0036](../redaction/0036-released-bodies-are-clean-markdown.md)), and subject masking running
  on every message ([ADR-0003](../redaction/0003-subject-masking.md)) are where that coupling is
  live.
- Crash-injection testing ([ADR-0045](./0045-crash-injection-testing.md)) is a different subject
  under test. A property generates ordinary inputs and checks a rule over the results. The crash
  harness generates crash points inside operation sequences and checks recovery afterward.
- The requirements this record places on tooling, a fixed case count and seed for gating runs, a
  failing input that is stored and replayed, and a longer search on the same definitions, are met as
  [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) decides. That record also decides
  what keeps a stored failing input meaningful after its generator is edited.
- A generator that never reaches the case is checked two ways that catch different things.
  [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md)'s generator report fails a run when
  a stated mix of inputs is not reached, which catches a generator that misses a kind of input
  somebody named. [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md)'s mutation
  demonstration catches a test that stays green with its mechanism removed, whatever the reason, but
  it reaches only the tests of a control.
