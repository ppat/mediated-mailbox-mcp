// Registers happy-dom's browser globals before any test module loads, at a URL with a real origin,
// because the router builds URLs against the location's origin (ADR-0064). bunfig.toml preloads this,
// and no test imports it.
import { GlobalRegistrator } from "@happy-dom/global-registrator";

// A stylesheet a test's document links is never fetched, since no server answers the origin, as the
// guide's windows link the bundle's stylesheet.
GlobalRegistrator.register({
  url: "https://ui.mediated-mailbox.test/",
  settings: { disableCSSFileLoading: true, handleDisabledFileLoadingAsSuccess: true },
});
