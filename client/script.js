const socket = new WebSocket("ws://localhost:8080/ws");
window.socket = socket;

const button = document.querySelector("#send-button");
const message = document.querySelector("#message");

socket.addEventListener("error", (event) => {
  console.log(event);
});

socket.addEventListener("open", () => {
  console.log("event register");

  button.addEventListener("click", () => {
    socket.send(message.value);
  });
});

socket.addEventListener("close", (event) => {
  console.log(event);
});
