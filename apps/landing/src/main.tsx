
  import { createRoot } from "react-dom/client";
  import App from "./app/App.tsx";
  import { initErrorReporting } from "./report-error";
  import "./styles/index.css";

  initErrorReporting();

  createRoot(document.getElementById("root")!).render(<App />);
  