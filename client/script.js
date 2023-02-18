const socket = new WebSocket("ws://localhost:8080/ws");
window.socket = socket;

const button = document.querySelector("#send-button");
const input = document.querySelector("#input");
const chat = document.querySelector("#chat");

socket.addEventListener("error", (event) => {
  console.log(event);
});

socket.addEventListener("open", () => {
  console.log("event register");

  socket.send("ping");

  button.addEventListener("click", () => {
    socket.send(input.value);
  });
});

socket.addEventListener("message", (ev) => {
  const message = document.createElement("p");
  message.appendChild(document.createTextNode(ev.data));
  chat.appendChild(message);
});

socket.addEventListener("close", (event) => {
  console.log(event);
});
