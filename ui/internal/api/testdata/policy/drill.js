fetch("https://drill.example/policy").then(() => {
  document.title = "the cross-origin fetch was answered";
});
