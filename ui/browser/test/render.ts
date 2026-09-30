// Mounting a component tree for a test, and letting its reads and effects settle.
import { render, type VNode } from "preact";
import { act } from "preact/test-utils";

export type Mounted = { root: HTMLElement; unmount: () => void };

export function mount(tree: VNode): Mounted {
  const root = document.createElement("div");
  document.body.append(root);
  render(tree, root);
  return {
    root,
    unmount: () => {
      render(null, root);
      root.remove();
    },
  };
}

// settle lets every pending read answer and every effect run, by turning the task queue over a few
// times inside act.
export async function settle(): Promise<void> {
  for (let i = 0; i < 5; i++) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  }
}

// at moves the browser to a path before a routed tree mounts.
export function at(path: string): void {
  history.replaceState(null, "", path);
}
