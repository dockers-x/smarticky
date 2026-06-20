import { mount } from "svelte";
import App from "./App.svelte";
import "./lib/styles/tokens.css";
import "./lib/styles/global.css";

const app = mount(App, {
  target: document.getElementById("smarticky-app") as HTMLElement,
});

export default app;
