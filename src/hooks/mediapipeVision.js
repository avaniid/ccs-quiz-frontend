const WASM_URL = "https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@1.0.1/wasm";

let filesetPromise = null;


export function getVisionFileset() {
  if (!filesetPromise) {
    filesetPromise = import("@mediapipe/tasks-vision").then(
      ({ FilesetResolver }) => FilesetResolver.forVisionTasks(WASM_URL)
    );
  }
  return filesetPromise;
}
