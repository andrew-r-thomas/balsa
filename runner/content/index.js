const startSim = () => {
	const sim_button = document.getElementById("sim_button")
	// TODO: maybe add some spinny circle thing here

	const nis = document.getElementById("node_infos")
	nis.innerHTML = ""

	const socket = new WebSocket("ws://localhost:8080/ws")
	socket.addEventListener("open", _ => {
		socket.send(JSON.stringify({ cmd: "start" }))

		sim_button.onclick = () => {
			socket.close()
			sim_button.textContent = "start sim"
			sim_button.onclick = startSim
		}
		sim_button.textContent = "stop sim"
	})

	socket.addEventListener("message", (event) => {
		const data = JSON.parse(event.data)
		switch (data.msg_type) {
			case "sim_started":
				for (const id of data.payload.ids) {
					console.log(id)
					const ni = document.createElement("li")
					ni.id = id

					const idEl = document.createElement("p")
					idEl.id = id + "/id"
					idEl.textContent = id
					ni.appendChild(idEl)

					const stateEl = document.createElement("p")
					stateEl.id = id + "/state"
					ni.appendChild(stateEl)

					const leaderEl = document.createElement("p")
					leaderEl.id = id + "/leader"
					ni.appendChild(leaderEl)

					const termEl = document.createElement("p")
					termEl.id = id + "/term"
					ni.appendChild(termEl)

					nis.appendChild(ni)
				}
				break
			case "state_update":
				const node = document.getElementById(`${data.payload.node}`)
				const state = document.getElementById(`${data.payload.node}/state`)
				const leader = document.getElementById(`${data.payload.node}/leader`)
				const term = document.getElementById(`${data.payload.node}/term`)

				state.textContent = "state: " + data.payload.state
				leader.textContent = "leader: " + data.payload.leader
				term.textContent = "term: " + data.payload.term

				switch (data.payload.state) {
					case "follower":
						node.style.backgroundColor = "white"
						break
					case "candidate":
						node.style.backgroundColor = "orange"
						break
					case "leader":
						node.style.backgroundColor = "green"
						break
					default:
						break
				}
				break
			default:
				console.log("ahhh!!!")
				break
		}
	})
}
