// Mounting a component tree for a test, and letting its reads and effects settle.
import { options, render, type VNode } from "preact";
import { act } from "preact/test-utils";

export type Mounted = { root: HTMLElement; unmount: () => void };

// mount renders the tree inside act, so the effects of its first render run before mount returns.
// Rendered outside act, preact would leave them to the browser's next frame, which under the DOM shim
// is a real task whose turn races the test.
export function mount(tree: VNode): Mounted {
  const root = document.createElement("div");
  document.body.append(root);
  void act(() => render(tree, root));
  return {
    root,
    unmount: () => {
      render(null, root);
      root.remove();
    },
  };
}

// rounds bounds settle. A tree still rendering after this many rounds is rendering in a loop.
const rounds = 50;

// settle lets every pending read answer and every effect run, and returns once nothing is pending.
// Each round turns the task queue once inside act. The turn runs every microtask queued before it and
// every one those queue, and the recorded fetch answers from memory within microtasks
// (test/fixtures/fetch.ts), so every read in flight lands in that turn. act then renders what the
// answers changed and runs the effects those renders queued, and an effect may start another read. A
// round that renders nothing leaves nothing pending, since only a render queues an effect, so settle
// returns after it. A read a test holds open on purpose stays open, as it should.
export async function settle(): Promise<void> {
  for (let round = 0; round < rounds; round++) {
    let rendered = false;
    const diffed: ((vnode: VNode) => void) | undefined = Object.getOwnPropertyDescriptor(
      options,
      "diffed",
    )?.value;
    options.diffed = (vnode: VNode) => {
      rendered = true;
      diffed?.(vnode);
    };
    try {
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0));
      });
    } finally {
      options.diffed = diffed;
    }
    if (!rendered) {
      return;
    }
  }
  throw new Error(`settle: the tree still renders after ${rounds} rounds`);
}

// at moves the browser to a path before a routed tree mounts.
export function at(path: string): void {
  history.replaceState(null, "", path);
}
