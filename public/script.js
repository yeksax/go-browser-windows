const canvas = document.querySelector('#canvas')
const ctx = canvas.getContext('2d')
let socket = new WebSocket('ws://localhost:8080/ws')

let width, height, x, y, id
let hasInitialized = false
let lines = []
let points = []
let balls = []

setInterval(() => {
  let hasChanged = false
  let tmp1 = [width, height, x, y]
  updateValues()

  let tmp2 = [width, height, x, y]

  for (let i = 0; i < tmp1.length; i++) {
    if (tmp1[i] !== tmp2[i]) {
      hasChanged = true
    }
  }

  if (hasChanged) {
    notifyServer()
  }
}, 1000 / 24)

window.addEventListener('beforeunload', () => {
  socket.send(JSON.stringify({
    type: "close-window",
    data: {
      id
    }
  }))
})

socket.addEventListener('close', () => {
  socket = new WebSocket('ws://localhost:8080/ws')
  hasInitialized = false
})

socket.addEventListener('message', (event) => {
  const raw_data = (JSON.parse(event.data))

  const data = raw_data.data
  const type = raw_data.type

  if (type === "new-window") {
    if (!id) {
      id = data.id
    }
  }

  if (type === "polygon") {
    lines = data.lines
    points = data.points
  }

  if (type === "balls") {
    balls = data
  }
})


function updateValues() {
  width = window.innerWidth
  height = window.innerHeight
  x = window.screenLeft
  y = window.screenTop

  if (!hasInitialized) {
    createWindow()
    draw()
    hasInitialized = true
  }
}

function createWindow() {
  socket.send(JSON.stringify({
    type: "new-window",
    data: {
      width,
      height,
      x,
      y
    }
  }))
}

function notifyServer() {
  socket.send(JSON.stringify({
    type: "update-window",
    data: {
      id,
      width,
      height,
      x,
      y
    }
  }))
}

function draw() {
  canvas.width = width
  canvas.height = height

  ctx.fillStyle = 'black'
  ctx.fillRect(0, 0, width, height)

  ctx.strokeStyle = 'white'
  for (let i = 0; i < lines.length; i++) {
    ctx.beginPath()
    ctx.moveTo(lines[i].from.x * .1, lines[i].from.y * .1)
    ctx.lineTo(lines[i].to.x * .1, lines[i].to.y * .1)
    ctx.stroke()
    ctx.closePath()
  }

  for (let i = 0; i < balls.length; i++) {
    ctx.beginPath()
    ctx.fillStyle = balls[i].color
    ctx.arc(balls[i].position.x - x, balls[i].position.y - y, balls[i].radius, 0, 2 * Math.PI)
    ctx.fill()
    ctx.closePath()
  }

  for (let i = 0; i < balls.length; i++) {
    ctx.beginPath()
    ctx.fillStyle = balls[i].color
    ctx.arc(balls[i].position.x * .1, balls[i].position.y * .1, balls[i].radius * .1, 0, 2 * Math.PI)
    ctx.fill()
    ctx.closePath()
  }

  requestAnimationFrame(draw)
}

document.addEventListener('click', (event) => {
  socket.send(JSON.stringify({
    type: "new-ball",
    data: {
      position: {
        x: event.clientX + x,
        y: event.clientY + y
      },
      velocity: {
        x: 0,
        y: 0
      },
      radius: 10,
      color: '#' + Math.floor(Math.random() * 16777215).toString(16),
    }
  }))
})
