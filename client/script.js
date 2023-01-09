const socket = new WebSocket("ws://localhost:8080/ws");

socket.addEventListener("error", (event) => {
  console.log(event);
});

socket.addEventListener("open", () => {
  console.log("Connection has been succesfully opened");

  socket.send("Hello from the browser!");
});

socket.addEventListener("close", (event) => {
  console.log(event);
});
