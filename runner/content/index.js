const startSim = async () => {
	const socket = new WebSocket("ws://localhost:8080/ws")
	socket.addEventListener("open", _ => {
		socket.send(JSON.stringify({ cmd: "start" }))
	})

	socket.addEventListener("message", (event) => {
		console.log(JSON.parse(event.data).payload.log)
	})
}
