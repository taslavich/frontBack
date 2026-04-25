import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import { createRoot } from "react-dom/client";
import App from "./App.tsx";
import "./index.css";

// Global toggle for surfacing API error toasts in the UI.
// Flip to `false` to silence error toasts (errors will still log to console).
(window as any).error_showed = true;

createRoot(document.getElementById("root")!).render(<App />);
