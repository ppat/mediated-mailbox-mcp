// Violation file. Suppressions of the signal rule that ast-grep honours but that are not the form ADR-0063
// sanctions, so the search for suppression directives reports each (ADR-0072). Each shares a line with the
// read, each names the field on the line above, and the last two silence more rules than the signal rule.
// Nothing imports this module.
type Row = { value: string };

export function Cells(props: { row: Row }) {
  return (
    <tr>
      <td>
        {
          // Row.value is the contract field, not a signal.
          props.row.value /* want suppression "ast-grep-ignore" */ // ast-grep-ignore: signal-value-in-render
        }
      </td>
      <td>
        {
          // Row.value is the contract field, not a signal.
          props.row.value /* want suppression "ast-grep-ignore" */ // ast-grep-ignore: signal-value-in-render, markup-prop-tsx
        }
      </td>
      <td>
        {
          // Row.value is the contract field, not a signal.
          props.row.value /* want suppression "ast-grep-ignore" */ // ast-grep-ignore
        }
      </td>
    </tr>
  );
}
