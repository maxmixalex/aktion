function mountAktionDemo() {
  const element = document.getElementById("aktion-cli-demo");
  if (!element || element.dataset.mounted === "true" || typeof AsciinemaPlayer === "undefined") return;

  element.dataset.mounted = "true";
  AsciinemaPlayer.create(element.dataset.cast, element, {
    cols: 140,
    rows: 30,
    fit: "width",
    preload: true,
    idleTimeLimit: 1.5,
    theme: "auto/asciinema",
    cursorMode: "hidden"
  });
}

if (typeof document$ !== "undefined") {
  document$.subscribe(mountAktionDemo);
} else {
  document.addEventListener("DOMContentLoaded", mountAktionDemo);
}
