import { mount } from "svelte";
import App from "./App.svelte";
import "./App.css";

mount(App, {
  target: document.getElementById("root")!   // <-- the exclamation ensures a non‑null target
});
