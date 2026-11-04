import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App.jsx";
import { DialogProvider, ToastProvider } from "@kit";
import "@kit/kit.css";
import "./styles.css";

createRoot(document.getElementById("root")).render(
  <ToastProvider><DialogProvider><App /></DialogProvider></ToastProvider>
);
