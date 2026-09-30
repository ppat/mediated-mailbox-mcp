// The group-by control of docs/UI.md section 6, on levels 1 and 2. It offers the dataset's groupable
// dimensions in the registry's order and writes group=.
import { descriptor, href, withGroup, type View } from "../app/url.ts";

export function GroupBy(props: { account: string; view: View; screen?: string }) {
  const { view } = props;
  if (view.level !== "1" && view.level !== "2") {
    return null;
  }
  const groupable = descriptor(view.dataset).dimensions.filter((d) => d.groupable);
  return (
    <div class="groupby" role="group" aria-label="Group by">
      <span class="label">group by</span>
      {groupable.map((d) => (
        <a
          key={d.name}
          class="preset"
          href={href(props.account, withGroup(view, d.name), props.screen)}
          aria-current={view.group === d.name ? "true" : undefined}
        >
          {d.wording}
        </a>
      ))}
    </div>
  );
}
