// Violation file. Each line below reaches for one of the test runner's substitution helpers, which the
// browser lint bans (ADR-0064, ADR-0072). It is not named as a test, so bun test never runs it.
import { mock } from "bun:test"; // want oxlint "no-restricted-imports.*'mock'"
import { spyOn } from "bun:test"; // want oxlint "no-restricted-imports.*'spyOn'"
import { jest } from "bun:test"; // want oxlint "no-restricted-imports.*'jest'"
import { vi } from "bun:test"; // want oxlint "no-restricted-imports.*'vi'"
import * as runner from "bun:test"; // want oxlint "no-restricted-imports.*\\* import"

const required = require("bun:test"); // want oxlint "no-require-imports"
const loaded = import("bun:test"); // want ast-grep "runner-dynamic-import-ts"
const other = import("preact");

export const helpers = [mock, spyOn, jest, vi, runner, required, loaded, other];
