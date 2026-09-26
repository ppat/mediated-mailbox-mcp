# Changelog

## [0.1.0](https://github.com/ppat/mediated-mailbox-mcp/compare/v0.0.1...v0.1.0) (2026-09-26)


### 🛠 Improvements

* accounts and their credentials live in the database, set up through the UI ([#170](https://github.com/ppat/mediated-mailbox-mcp/issues/170)) ([f8e1ecd](https://github.com/ppat/mediated-mailbox-mcp/commit/f8e1ecd35a3efc5403503084cccada17181f28b5))
* **agents:** a mutation patch sits with the control it demonstrates ([#147](https://github.com/ppat/mediated-mailbox-mcp/issues/147)) ([74f926d](https://github.com/ppat/mediated-mailbox-mcp/commit/74f926d12447272b672d2f481da9ffbd1de981b7)), refs [#146](https://github.com/ppat/mediated-mailbox-mcp/issues/146)
* close the coherence gaps between the document set and the tickets ([#120](https://github.com/ppat/mediated-mailbox-mcp/issues/120)) ([8da026a](https://github.com/ppat/mediated-mailbox-mcp/commit/8da026ac861cafd9da65635bebaab339c440eff0))
* establish the ticket format, its template and its labels ([#67](https://github.com/ppat/mediated-mailbox-mcp/issues/67)) ([26f0890](https://github.com/ppat/mediated-mailbox-mcp/commit/26f0890904236558ab81ab6b50025dc9561d4e81))
* permanent proofs for the tooling checks still pending ([#131](https://github.com/ppat/mediated-mailbox-mcp/issues/131)) ([1d3802b](https://github.com/ppat/mediated-mailbox-mcp/commit/1d3802b2fcd3a10de268dd02b74d9eb874e6fe7a)), refs [#90](https://github.com/ppat/mediated-mailbox-mcp/issues/90)
* policy is managed through the UI in unit M8, and no document cites a deprecated record ([#175](https://github.com/ppat/mediated-mailbox-mcp/issues/175)) ([d754eaa](https://github.com/ppat/mediated-mailbox-mcp/commit/d754eaa36efa933fba09f95f57d7e95a59d933e4))
* reconcile the document set with the merged tooling, and track tickets under one epic ([#119](https://github.com/ppat/mediated-mailbox-mcp/issues/119)) ([935c369](https://github.com/ppat/mediated-mailbox-mcp/commit/935c3692a15d684706e4053df4cbd401521cf41e))
* record how every deployable takes its configuration ([#168](https://github.com/ppat/mediated-mailbox-mcp/issues/168)) ([3541a9f](https://github.com/ppat/mediated-mailbox-mcp/commit/3541a9f78e020491405f8536c52e7987a665ad1f))
* record the read path's UI screens shipping before production point 1 ([#163](https://github.com/ppat/mediated-mailbox-mcp/issues/163)) ([b4181c0](https://github.com/ppat/mediated-mailbox-mcp/commit/b4181c0ab91a6701c43e53a1a6a62aafa1ac0e37))
* the go vet analyser refuses package-level state in a pure core ([#149](https://github.com/ppat/mediated-mailbox-mcp/issues/149)) ([6f76e7f](https://github.com/ppat/mediated-mailbox-mcp/commit/6f76e7f9590ba9f96c0511a855adc3bc78d7f4b3)), refs [#148](https://github.com/ppat/mediated-mailbox-mcp/issues/148)
* the golden-file helper ([#162](https://github.com/ppat/mediated-mailbox-mcp/issues/162)) ([8515de1](https://github.com/ppat/mediated-mailbox-mcp/commit/8515de1354a40d27d7ebb3f7394e6e40bad5bfd0))
* the marker text and the synthetic fixtures ([#141](https://github.com/ppat/mediated-mailbox-mcp/issues/141)) ([da74ef6](https://github.com/ppat/mediated-mailbox-mcp/commit/da74ef6dc8f0b0e30507275536a6abefd7f5c16f)), refs [#122](https://github.com/ppat/mediated-mailbox-mcp/issues/122)
* the mutation demonstration runner ([#132](https://github.com/ppat/mediated-mailbox-mcp/issues/132)) ([301bf86](https://github.com/ppat/mediated-mailbox-mcp/commit/301bf864ac0ee3e0912c5b984baf273821f4c32a)), refs [#69](https://github.com/ppat/mediated-mailbox-mcp/issues/69)


### ✨ Features

* accounts, OAuth clients and sealed credentials in the database, with credential and accountload ([#178](https://github.com/ppat/mediated-mailbox-mcp/issues/178)) ([d5cec07](https://github.com/ppat/mediated-mailbox-mcp/commit/d5cec07d092f8c1034f2b6b26f12f725220a8aab))
* body sanitization to Markdown and the serve-time pattern check ([#152](https://github.com/ppat/mediated-mailbox-mcp/issues/152)) ([21e9d69](https://github.com/ppat/mediated-mailbox-mcp/commit/21e9d69e3bb3339e885e1346a51e2716b4fd9147)), refs [#87](https://github.com/ppat/mediated-mailbox-mcp/issues/87)
* database leases, the Gmail cost profile, the rate metrics and the alerting rules ([#157](https://github.com/ppat/mediated-mailbox-mcp/issues/157)) ([f370fc7](https://github.com/ppat/mediated-mailbox-mcp/commit/f370fc7003682874cf49aeaa51a750c59d5b54d7))
* installed-app OAuth, credentials as mounted files and rotation write-back's application half ([#155](https://github.com/ppat/mediated-mailbox-mcp/issues/155)) ([13d4e57](https://github.com/ppat/mediated-mailbox-mcp/commit/13d4e572a5eb946f8e0585a6714bae6892163e09)), refs [#76](https://github.com/ppat/mediated-mailbox-mcp/issues/76)
* one runaway rule for every provider, and a rate target an account can lower ([#161](https://github.com/ppat/mediated-mailbox-mcp/issues/161)) ([4bb610a](https://github.com/ppat/mediated-mailbox-mcp/commit/4bb610ac4d22c05a47ecd290ea2730a6ed08aa5d))
* the canonical model, the Provider Port, the provider fake and the contract suite ([#154](https://github.com/ppat/mediated-mailbox-mcp/issues/154)) ([e1d6517](https://github.com/ppat/mediated-mailbox-mcp/commit/e1d651795bec0960d2cd4bd81ddf89171e05e85a)), refs [#75](https://github.com/ppat/mediated-mailbox-mcp/issues/75)
* the configuration library, layering defaults, one optional YAML file, environment variables and flags ([#177](https://github.com/ppat/mediated-mailbox-mcp/issues/177)) ([8351ba6](https://github.com/ppat/mediated-mailbox-mcp/commit/8351ba6883846f9900dbebce100bf554618fa035)), refs [#166](https://github.com/ppat/mediated-mailbox-mcp/issues/166)
* the Content Scanner's first two tiers and subject masking ([#151](https://github.com/ppat/mediated-mailbox-mcp/issues/151)) ([f2d7e97](https://github.com/ppat/mediated-mailbox-mcp/commit/f2d7e976d27371d8b9b7814124db2afc01cf6841)), refs [#80](https://github.com/ppat/mediated-mailbox-mcp/issues/80)
* the Gmail adapter, passing the contract suite ([#158](https://github.com/ppat/mediated-mailbox-mcp/issues/158)) ([54bfa1f](https://github.com/ppat/mediated-mailbox-mcp/commit/54bfa1f0860ea72563383fe68d7bcd7ae8d4efdc))
* the Mutation Authorizer's matrix ([#150](https://github.com/ppat/mediated-mailbox-mcp/issues/150)) ([3f5ff01](https://github.com/ppat/mediated-mailbox-mcp/commit/3f5ff01481214a562de3e238de58a23948656fd3)), refs [#91](https://github.com/ppat/mediated-mailbox-mcp/issues/91)
* the policy snapshot's validation and atomic swap ([#143](https://github.com/ppat/mediated-mailbox-mcp/issues/143)) ([816a0eb](https://github.com/ppat/mediated-mailbox-mcp/commit/816a0ebed813c2962ab8dec202bac544755b3be4)), refs [#72](https://github.com/ppat/mediated-mailbox-mcp/issues/72)
* the rate controller's pure rules for the target, the hard cap, AIMD and the priority classes ([#156](https://github.com/ppat/mediated-mailbox-mcp/issues/156)) ([c86a4e6](https://github.com/ppat/mediated-mailbox-mcp/commit/c86a4e663d77b549801992f056405bfeb1907c99))
* the Redaction Gate's matrix and its fetch-time deny branches ([#145](https://github.com/ppat/mediated-mailbox-mcp/issues/145)) ([c231a61](https://github.com/ppat/mediated-mailbox-mcp/commit/c231a61207917e894e49fe9ff366f1198a605f5e)), refs [#83](https://github.com/ppat/mediated-mailbox-mcp/issues/83)
* the scan gate's composite predicate ([#167](https://github.com/ppat/mediated-mailbox-mcp/issues/167)) ([7b07f31](https://github.com/ppat/mediated-mailbox-mcp/commit/7b07f312f7b9d4f1cf041d09829dde3c6a3b3f30)), refs [#82](https://github.com/ppat/mediated-mailbox-mcp/issues/82)
* the schema, the migration chain, the runtime roles and the transaction helper ([#153](https://github.com/ppat/mediated-mailbox-mcp/issues/153)) ([8960ce4](https://github.com/ppat/mediated-mailbox-mcp/commit/8960ce4aa4781c5c9add0071071574aeb312aa87)), refs [#70](https://github.com/ppat/mediated-mailbox-mcp/issues/70)
* the sender classifier ([#144](https://github.com/ppat/mediated-mailbox-mcp/issues/144)) ([5115a2f](https://github.com/ppat/mediated-mailbox-mcp/commit/5115a2fc8f8ff500e4bab546dd3b53faad0ef219)), refs [#79](https://github.com/ppat/mediated-mailbox-mcp/issues/79)
* the sensitivity types and the property-testing harness ([#142](https://github.com/ppat/mediated-mailbox-mcp/issues/142)) ([8c3eaf9](https://github.com/ppat/mediated-mailbox-mcp/commit/8c3eaf961f2be75dbb32d1aec42ec7879bdcf860)), refs [#71](https://github.com/ppat/mediated-mailbox-mcp/issues/71)
* the shared policy loader with its reload-failure alarm, used first by backfill ([#160](https://github.com/ppat/mediated-mailbox-mcp/issues/160)) ([48a1412](https://github.com/ppat/mediated-mailbox-mcp/commit/48a141264e08875f137205a87304045fc60bfff3))


### 🚀 Enhancements + Bug Fixes

* make go.mod and go.sum tidy, and fail the build when they drift ([#134](https://github.com/ppat/mediated-mailbox-mcp/issues/134)) ([eb34bff](https://github.com/ppat/mediated-mailbox-mcp/commit/eb34bff2fe72549c4f33040b4c06cace4dd47807)), refs [#133](https://github.com/ppat/mediated-mailbox-mcp/issues/133)
* update digest github.com/wasilibs/go-pgquery (318158a -&gt; 81f9919) ([#115](https://github.com/ppat/mediated-mailbox-mcp/issues/115)) ([a0f2943](https://github.com/ppat/mediated-mailbox-mcp/commit/a0f2943c8d9b778ff8f6b098f3efb77bc425d41d))

## 0.0.1 (2026-09-16)


### 🛠 Improvements

* add the update-docs skill, per-document rules, and the standing update duty ([#4](https://github.com/ppat/mediated-mailbox-mcp/issues/4)) ([4e798e3](https://github.com/ppat/mediated-mailbox-mcp/commit/4e798e33175afd9fdcc3316bb7ab6aedb2ab6e3e))
* enforce the prose target form and correct platform-specific drift ([#52](https://github.com/ppat/mediated-mailbox-mcp/issues/52)) ([23293ca](https://github.com/ppat/mediated-mailbox-mcp/commit/23293cac217476bf0e026742a567c81576dd783f))
* harden the update-docs skill from adversarial review findings ([#10](https://github.com/ppat/mediated-mailbox-mcp/issues/10)) ([ddc3a1a](https://github.com/ppat/mediated-mailbox-mcp/commit/ddc3a1a900da3a9b7f76b257b2dfe3e78588954f))
* mint ADR-0030 — the serving layer is an API; MCP is a thin protocol adapter ([#5](https://github.com/ppat/mediated-mailbox-mcp/issues/5)) ([d9c0172](https://github.com/ppat/mediated-mailbox-mcp/commit/d9c017277bb5c394385c16e32b137b91e56b2f07))
* mint ADR-0031 — every mutating operation is dry-runnable ([#7](https://github.com/ppat/mediated-mailbox-mcp/issues/7)) ([c703310](https://github.com/ppat/mediated-mailbox-mcp/commit/c703310b65fba13cbdc08f6f9706c5e56f09d7ec))
* mint ADR-0032 — all validation precedes the first write; plans re-validate at apply and expire ([#9](https://github.com/ppat/mediated-mailbox-mcp/issues/9)) ([4f92aa1](https://github.com/ppat/mediated-mailbox-mcp/commit/4f92aa1c3e466936164504f7b1d817acb5cf60c0))
* mint ADR-0033 — timestamps on the client surface are UTC-only, non-UTC input rejected ([#11](https://github.com/ppat/mediated-mailbox-mcp/issues/11)) ([4ac7b4d](https://github.com/ppat/mediated-mailbox-mcp/commit/4ac7b4daf777f553a52675f9d483863f0b7ef8c0))
* mint ADR-0034 — one read-only system-status operation over recorded per-account state ([#12](https://github.com/ppat/mediated-mailbox-mcp/issues/12)) ([ce70002](https://github.com/ppat/mediated-mailbox-mcp/commit/ce70002a258ae855b502b7361aa2bc2e0d65239a))
* mint ADR-0035 — every required identifier is discoverable; accounts gain a listing ([#13](https://github.com/ppat/mediated-mailbox-mcp/issues/13)) ([4aea993](https://github.com/ppat/mediated-mailbox-mcp/commit/4aea9932233c8e0e4195ceebc8dfe9a74bd3489d))
* mint ADR-0037 — delisting is a designed transition, messages marked pending scan ([#19](https://github.com/ppat/mediated-mailbox-mcp/issues/19)) ([4329e42](https://github.com/ppat/mediated-mailbox-mcp/commit/4329e422c91bbf34ef1afe76812f4a350c7027ab))
* mint ADR-0040 — cores are pure and decisions are values; thin impure shells enact them ([#33](https://github.com/ppat/mediated-mailbox-mcp/issues/33)) ([515532f](https://github.com/ppat/mediated-mailbox-mcp/commit/515532f51e95070446cb864f5a5e0917d5e0d9c1))
* mint ADR-0041 — policy arrives as an immutable snapshot, taken once per unit of work ([#41](https://github.com/ppat/mediated-mailbox-mcp/issues/41)) ([923fd09](https://github.com/ppat/mediated-mailbox-mcp/commit/923fd09d2206ff80538233dda971a0ce1aa34969))
* mint ADR-0042 — the stack is Go end to end on the server, TypeScript only in the browser ([#34](https://github.com/ppat/mediated-mailbox-mcp/issues/34)) ([c812dad](https://github.com/ppat/mediated-mailbox-mcp/commit/c812dad07dad62ff2c762867feeb51a486711cb9))
* mint ADR-0043 — no mocking: tests run against the real dependency or a contract-tested fake ([#35](https://github.com/ppat/mediated-mailbox-mcp/issues/35)) ([31afea0](https://github.com/ppat/mediated-mailbox-mcp/commit/31afea08e32da06168297a6dbd12981d9744a456))
* mint ADR-0044, ADR-0046, ADR-0055 — layered tests, the mutation obligation, property-based testing ([#49](https://github.com/ppat/mediated-mailbox-mcp/issues/49)) ([f86173e](https://github.com/ppat/mediated-mailbox-mcp/commit/f86173ed5a3f48f5c106e0319b9b7dd0b4070184))
* mint ADR-0045 — crash-injection stateful testing is aimed where silent failure meets hard-to-reverse damage ([#36](https://github.com/ppat/mediated-mailbox-mcp/issues/36)) ([031cd7b](https://github.com/ppat/mediated-mailbox-mcp/commit/031cd7bfb728e6955a8a3aafa51a06d9f69bed68))
* mint ADR-0047 and ADR-0048 — schema-first data access; forward-only migrations ([#37](https://github.com/ppat/mediated-mailbox-mcp/issues/37)) ([c80156a](https://github.com/ppat/mediated-mailbox-mcp/commit/c80156a714f50308f6e1b9e486ccc2655979d4b7))
* mint ADR-0049 — one image per deployable, all moving in lockstep ([#38](https://github.com/ppat/mediated-mailbox-mcp/issues/38)) ([39a0568](https://github.com/ppat/mediated-mailbox-mcp/commit/39a056824628912ca51359197cfd6615ae8d2f5b))
* mint ADR-0050 — shared code is pure, or it is a narrow, named exception ([#39](https://github.com/ppat/mediated-mailbox-mcp/issues/39)) ([2318ffa](https://github.com/ppat/mediated-mailbox-mcp/commit/2318ffafa6ae0809abc8bf33a62646f97e609d2b))
* mint ADR-0051 — the app knows its environment contract, never its platform ([#42](https://github.com/ppat/mediated-mailbox-mcp/issues/42)) ([d3cd332](https://github.com/ppat/mediated-mailbox-mcp/commit/d3cd332b497ff0474431519e6ba7323b389c1de6))
* mint ADR-0052 and outcome O6 — the Kubernetes deployment is one Helm chart; the system is deployable ([#45](https://github.com/ppat/mediated-mailbox-mcp/issues/45)) ([7be1536](https://github.com/ppat/mediated-mailbox-mcp/commit/7be153631557e61a67cfb5ccd9a04ce4b3c873ce))
* mint ADR-0053 — both client roots are generated from one operation registry ([#40](https://github.com/ppat/mediated-mailbox-mcp/issues/40)) ([d654121](https://github.com/ppat/mediated-mailbox-mcp/commit/d65412182df5f0a46f417febf8ec94aaed7fc61c))
* mint ADR-0054 — one flat repository holds everything; one convention names what it publishes ([#51](https://github.com/ppat/mediated-mailbox-mcp/issues/51)) ([12db79d](https://github.com/ppat/mediated-mailbox-mcp/commit/12db79dc7839a43c30a100118782f55386eefaa1))
* mint ADR-0063 to ADR-0065 — Preact, browser tests under bun, the contract pipeline, amending ADR-0046 and ADR-0060 ([#55](https://github.com/ppat/mediated-mailbox-mcp/issues/55)) ([796e6e5](https://github.com/ppat/mediated-mailbox-mcp/commit/796e6e55a9470fa489f49f4bbbb0043a0f72c794))
* mint ADR-0066 to ADR-0068 — the data layer's tooling, and the corrections choosing it surfaced ([#56](https://github.com/ppat/mediated-mailbox-mcp/issues/56)) ([20a74f2](https://github.com/ppat/mediated-mailbox-mcp/commit/20a74f279038d2db728306ff122352edd164d7e2))
* mint ADR-0069 to ADR-0072 — the testing and static enforcement toolchain ([#58](https://github.com/ppat/mediated-mailbox-mcp/issues/58)) ([e62d47a](https://github.com/ppat/mediated-mailbox-mcp/commit/e62d47a95b76b1b8a9c69c933f1bcb91d6f83dda))
* partition the design into a rate-of-change document set with decision records ([#3](https://github.com/ppat/mediated-mailbox-mcp/issues/3)) ([2c8f0d2](https://github.com/ppat/mediated-mailbox-mcp/commit/2c8f0d2fd1a9ce191e91fab9974be585e3feb8a5))
* reconcile the roadmap to finish lines, three production points, and parallel build ([#65](https://github.com/ppat/mediated-mailbox-mcp/issues/65)) ([7aa8a6b](https://github.com/ppat/mediated-mailbox-mcp/commit/7aa8a6b719b26958aef3a90d4f7e9f0aa75c60cc))
* record outcome A4; ADR-0036 supersedes ADR-0029 — released bodies are clean Markdown ([#17](https://github.com/ppat/mediated-mailbox-mcp/issues/17)) ([fe363e7](https://github.com/ppat/mediated-mailbox-mcp/commit/fe363e792956a4dc9480112b28fd4dae702c4bf9))
* record outcome C4 — the sensitive-sender list keeps pace with the mailbox ([#14](https://github.com/ppat/mediated-mailbox-mcp/issues/14)) ([a73eff8](https://github.com/ppat/mediated-mailbox-mcp/commit/a73eff8c62e828e10df7e6b1a317f7a2c8208031))
* record outcome G4 — the index tracks the live mailbox ([#15](https://github.com/ppat/mediated-mailbox-mcp/issues/15)) ([cc30178](https://github.com/ppat/mediated-mailbox-mcp/commit/cc30178f913f102a90f51fef9a1805415483bf44))
* record outcome O4 — the operator can see and steer the system ([#16](https://github.com/ppat/mediated-mailbox-mcp/issues/16)) ([917efbe](https://github.com/ppat/mediated-mailbox-mcp/commit/917efbee436a400ad380ef93d885ead156863863))
* record outcome O5 — clients can tell failures apart ([#21](https://github.com/ppat/mediated-mailbox-mcp/issues/21)) ([ae54ad5](https://github.com/ppat/mediated-mailbox-mcp/commit/ae54ad51f6b697e022165b9cc6a588fc8650a7e6))
* record the code layout in CLAUDE.md and the library READMEs, and reconcile the records it corrected ([#61](https://github.com/ppat/mediated-mailbox-mcp/issues/61)) ([bd95c33](https://github.com/ppat/mediated-mailbox-mcp/commit/bd95c338ff2ba283613f7437274f2e0ebed8bb22))
* record the evolvability pillar — concerns stay un-braided, components know only their contracts ([#6](https://github.com/ppat/mediated-mailbox-mcp/issues/6)) ([eae5925](https://github.com/ppat/mediated-mailbox-mcp/commit/eae5925d7845ee51c6c8900d847781fd66e96235))
* record the testing strategy — TESTING.md joins the set with its enforcement ([#50](https://github.com/ppat/mediated-mailbox-mcp/issues/50)) ([9fef28e](https://github.com/ppat/mediated-mailbox-mcp/commit/9fef28e42cbc683dfd04743f81be1005a6ce6b50))
* record the UI design — docs/UI.md joins the set, mint ADR-0056 to ADR-0062 ([#54](https://github.com/ppat/mediated-mailbox-mcp/issues/54)) ([86eaaeb](https://github.com/ppat/mediated-mailbox-mcp/commit/86eaaebacf58c06dd3e31363e117f6dd2f9ff0db))
* record three falsifier additions — calendar into C2, masking into O2, label-deletion into A3 ([#18](https://github.com/ppat/mediated-mailbox-mcp/issues/18)) ([af54bad](https://github.com/ppat/mediated-mailbox-mcp/commit/af54badaf9af3b88b2840faa41d59a0f6ab5f1c0))
* record which property-based tests are written and the tools that run them, in ADR-0055 and ADR-0069 ([#59](https://github.com/ppat/mediated-mailbox-mcp/issues/59)) ([dc33c32](https://github.com/ppat/mediated-mailbox-mcp/commit/dc33c3213afec14da823c93661d3b49a46afb045))
* recut the testing docs — TESTING.md assembles, each record holds one decision ([#53](https://github.com/ppat/mediated-mailbox-mcp/issues/53)) ([74966dc](https://github.com/ppat/mediated-mailbox-mcp/commit/74966dc9924dab5bb6d5ca585734567b90ec8bca))
* supersede ADR-0013 — credentials arrive as mounted files; rotation write-back is delegated ([#20](https://github.com/ppat/mediated-mailbox-mcp/issues/20)) ([b1486a1](https://github.com/ppat/mediated-mailbox-mcp/commit/b1486a1dbde0c2c5c63e9caefef55f8f4eceb509))
* the update-docs skill says how a tool selection is written into a record ([#57](https://github.com/ppat/mediated-mailbox-mcp/issues/57)) ([ab296fd](https://github.com/ppat/mediated-mailbox-mcp/commit/ab296fdc59a5aeff92d60ecd96ab5145af5f7f2f))


### ✨ Features

* initial design ([#1](https://github.com/ppat/mediated-mailbox-mcp/issues/1)) ([9582b05](https://github.com/ppat/mediated-mailbox-mcp/commit/9582b05de73b1a7b8f4004a9b65a8d1239edd00a))
