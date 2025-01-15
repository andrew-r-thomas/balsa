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
					stateEl.textContent = "state: follower"
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

				switch (data.payload.msg) {
					case "state":
						const state = document.getElementById(`${data.payload.node}/state`)
						state.textContent = "state: " + data.payload.val
						switch (data.payload.val) {
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
								console.error("invalid node state!")
								break
						}
						break
					case "leader":
						const leader = document.getElementById(`${data.payload.node}/leader`)
						leader.textContent = "leader: " + data.payload.val
						break
					case "term":
						const term = document.getElementById(`${data.payload.node}/term`)
						term.textContent = "term: " + data.payload.val
						break
					default:
						console.error("invalid update message!")
						break
				}
				break
			default:
				console.error("invalid message type!")
				break
		}
	})
}
