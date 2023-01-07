const socket = new WebSocket("ws://localhost:8080");

socket.addEventListener("open", () => {
  console.log("Connection has succesfully opened");
});
