# Proposed Improvements

The following is a concise, action‑oriented list of code improvements that would enhance clarity, maintainability, and robustness of `main.go`.

1. **Add detailed package documentation** – Provide a short package comment explaining the purpose of the CLI and its modes.
2. **Refactor `Change.Actions` handling** – Remove the unused second return value from `getActionDetails`. It only needs a symbol and action type.
3. **Extract color helpers into constants** – Define `ColorAdd`, `ColorDelete`, `ColorUpdate`, `ColorReplace`, `ColorComment` constants once for reuse.
4. **Improve `formatValue`** – Use `json.Marshal` (single line) for simple values and keep `MarshalIndent` for multi-line structures to reduce verbosity.
5. **Add test coverage** – Write unit tests for `isUnknown`, `forcesReplacement`, `getActionDetails`, and `renderResourceDiff`.
6. **Introduce a JSON output mode** – Add a `json` flag that writes the raw, filtered plan JSON for scripting use.
7. **Graceful exit codes** – Return exit code `0` on success, and non‑zero codes for parse or runtime errors.
8. **Improve flag handling** – Add `-h/--help` automatic help from `flag` package (already provided) but document usage in README.
9. **Rename `SummaryCounts.String`** – Change method name to `Summary()` to avoid conflict with `String()` interface.
10. **Add version flag** – Support `-v` or `--version` to print the binary version.
11. **Use `json.NewDecoder` for streaming** – In `parsePlan` use a streaming decoder to handle very large plans.
12. **Validate input JSON before parsing** – Detect and report malformed JSON earlier.
13. **Add `useColor` option to all modes** – Ensure `--no-color` works for `md` and `table` modes.
14. **Add logging** – Use the `log` package with a flag to enable verbose debug output.
15. **Documentation of CLI options** – Update README to include a detailed options table.

Implementing these changes will make the tool more user‑friendly, easier to test, and future‑proof.
