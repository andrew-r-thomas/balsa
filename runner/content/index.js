const startSim = () => {
	const b = document.getElementById("sim_button")

	const socket = new WebSocket("ws://localhost:8080/ws")
	socket.addEventListener("open", _ => {
		socket.send(JSON.stringify({ cmd: "start" }))
	})

	socket.addEventListener("message", (event) => {
		const data = JSON.parse(event.data)
		switch (data.msg_type) {
			case "sim_started":
				for (const id of data.payload.ids) {
					console.log(`id: ${id}`)
				}
				// TODO: update button
				break
			case "state_update":
				// console.log(`state update: ${JSON.stringify(data.payload)}`)
				// TODO: update dom
				break
			default:
				console.log("ahhh!!!")
				break
		}
	})
}
