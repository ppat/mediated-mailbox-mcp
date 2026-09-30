# Font files

The UI's two faces, IBM Plex Sans and IBM Plex Mono, self-hosted as
[docs/UI.md section 14.3](../../../../docs/UI.md#143-typography-and-spacing) requires. Each file is
the Latin-1 subset in woff2, copied unchanged from the `fonts/split/woff2/` directory of IBM's own
release archive. `LICENSE.txt` is the typeface's SIL Open Font License, byte for byte as both
archives carry it at their root, SHA-256
`7e6b2818edbd8f6a01ae80641cc8f16a51080d08fb4e532be3a0b6f74adb07da`. `.gitattributes` and the
pre-commit exclusions keep git and the text fixers from rewriting its CRLF line endings and trailing
whitespace.

| File | Release on `IBM/plex` | Archive SHA-256 | File SHA-256 |
| --- | --- | --- | --- |
| `IBMPlexSans-Regular-Latin1.woff2` | `@ibm/plex-sans@1.1.0`, `ibm-plex-sans.zip` | `fb365d910566e6d199cc2c15579a7dd9a267128e18431a394ed81f1970c69200` | `b5ad7bd39f996144915f0ad9849a90183b27d8c28ad97ed98af5b1bebc51f6b1` |
| `IBMPlexSans-Medium-Latin1.woff2` | `@ibm/plex-sans@1.1.0`, `ibm-plex-sans.zip` | `fb365d910566e6d199cc2c15579a7dd9a267128e18431a394ed81f1970c69200` | `b5610af04d0d4b5a14a621d96d974b993e945a065db1a8861918f69ef9321934` |
| `IBMPlexSans-SemiBold-Latin1.woff2` | `@ibm/plex-sans@1.1.0`, `ibm-plex-sans.zip` | `fb365d910566e6d199cc2c15579a7dd9a267128e18431a394ed81f1970c69200` | `fff0ab3a88b0b4aa0b693e4f0201359a15183b08e3fa5696d1918d8f0ade8ad5` |
| `IBMPlexMono-Regular-Latin1.woff2` | `@ibm/plex-mono@2.5.0`, `ibm-plex-mono.zip` | `6d23f01257663d8cc49a0d64c22ced630b79e0e2a0ac08a0da86e9a38bbc481c` | `e8993d946649b9d01abb1ed06d574b19d8ea3e66b5c3948602db335c44c18e56` |
| `IBMPlexMono-Medium-Latin1.woff2` | `@ibm/plex-mono@2.5.0`, `ibm-plex-mono.zip` | `6d23f01257663d8cc49a0d64c22ced630b79e0e2a0ac08a0da86e9a38bbc481c` | `41201b658a328b9d00368215c2f1102770f80b15952ab82631e4006255e6365d` |

Renovate does not track these files. A new release is taken by hand, by replacing a file and its
row here. `scripts/build.ts` copies every woff2 file in this directory into the bundle's `fonts/`,
and `src/app/theme.css` names each one in an `@font-face` rule.
