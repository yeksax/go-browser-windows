const canvas = document.querySelector('#canvas')
const ctx = canvas.getContext('2d')
const websocketURL = `ws://${window.location.host}/ws`

let socket = new WebSocket(websocketURL)

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

  hasChanged = tmp1[0] !== tmp2[0] || tmp1[1] !== tmp2[1] || tmp1[2] !== tmp2[2] || tmp1[3] !== tmp2[3]

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
  socket = new WebSocket(websocketURL)
  hasInitialized = false
})

socket.addEventListener('message', (event) => {
  const raw_data = (JSON.parse(event.data))

  const data = raw_data.data
  const type = raw_data.type

  if (type === "new-window") {
    if (!id) {
      id = data.id
      hasInitialized = true

      notifyServer()
      draw()
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

function updateValues() {
  width = window.innerWidth
  height = window.innerHeight
  x = window.screenLeft
  y = window.screenTop

  if (socket.readyState !== 1)
    return


  if (!hasInitialized) {
    createWindow()
  }
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
  ctx.fillStyle = 'white'
  for (let i = 0; i < lines.length; i++) {
    ctx.beginPath()
    ctx.moveTo(lines[i].from.x * .1, lines[i].from.y * .1)
    ctx.lineTo(lines[i].to.x * .1, lines[i].to.y * .1)
    ctx.stroke()
    ctx.closePath()
  }

  for (let i = 0; i < balls.length; i++) {
    ctx.beginPath()
    ctx.arc(balls[i].position.x - x, balls[i].position.y - y, balls[i].radius, 0, 2 * Math.PI)
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

function hslToHex(h, s, l) {
  l /= 100
  const a = s * Math.min(l, 1 - l) / 100
  const f = n => {
    const k = (n + h / 30) % 12
    const color = l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1)
    return Math.round(255 * color).toString(16).padStart(2, '0')
  }
  return `#${f(0)}${f(8)}${f(4)}`
}
