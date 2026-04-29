import { contextBridge } from "electron";

contextBridge.exposeInMainWorld("hedhuntr", {
  runtime: "electron",
  versions: {
    electron: process.versions.electron,
    node: process.versions.node,
    chrome: process.versions.chrome
  }
});
