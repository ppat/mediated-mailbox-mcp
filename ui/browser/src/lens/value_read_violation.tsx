// Violation file. A contract field named value, read in a rendering position. The first read carries the
// suppression ADR-0063 sanctions, in the form ADR-0072 states, with the field named on the line above the
// directive, so neither ast-grep nor the search for suppression directives may report it. The second read
// carries no suppression, so the signal rule reports it. Nothing imports this module.
type Row = { value: string };

export function Cell(props: { row: Row }) {
  return (
    <td title={props.row.value /* want ast-grep "signal-value-in-render" */}>
      {
        // Row.value is the contract field, not a signal.
        // ast-grep-ignore: signal-value-in-render
        props.row.value
      }
    </td>
  );
}
