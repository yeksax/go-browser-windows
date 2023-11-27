package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var app *gin.Engine

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Event struct {
	Type string `json:"type"`
	Data Window `json:"data"`
}

type BallEvent struct {
	Type string `json:"type"`
	Data Ball   `json:"data"`
}

type BallsEvent struct {
	Type string `json:"type"`
	Data []Ball `json:"data"`
}

type Vector2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Line struct {
	From Vector2D `json:"from"`
	To   Vector2D `json:"to"`
}

type Polygon struct {
	Points []Vector2D `json:"points"`
	Lines  []Line     `json:"lines"`
}

type Window struct {
	ID     int     `json:"id"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

type Ball struct {
	Position Vector2D `json:"position"`
	Velocity Vector2D `json:"velocity"`
	Radius   float64  `json:"radius"`
	Color    string   `json:"color"`
}

type Client struct {
	id     int
	conn   *websocket.Conn
	window *Window
}

func (v1 *Vector2D) Add(v2 Vector2D) {
	v1.X += v2.X
	v1.Y += v2.Y
}

func (v1 *Vector2D) Sub(v2 Vector2D) {
	v1.X -= v2.X
	v1.Y -= v2.Y
}

var balls = []Ball{}
var clients = map[int]Client{}
var polygon = Polygon{}
var currentClientId = 0

func WindowToPolygon(window Window) Polygon {
	polygon := Polygon{}

	polygon.Points = append(polygon.Points, Vector2D{X: window.X, Y: window.Y})
	polygon.Points = append(polygon.Points, Vector2D{X: window.X + window.Width, Y: window.Y})
	polygon.Points = append(polygon.Points, Vector2D{X: window.X + window.Width, Y: window.Y + window.Height})
	polygon.Points = append(polygon.Points, Vector2D{X: window.X, Y: window.Y + window.Height})

	return polygon
}

func MergePolygons(p1 Polygon, p2 Polygon) Polygon {
	return Polygon{
		Points: append(p1.Points, p2.Points...),
	}
}

func ComputeOrthogonalConvexHull(points []Vector2D) []Vector2D {
	// Sort points based on X and then Y
	sort.Slice(points, func(i, j int) bool {
		if points[i].X == points[j].X {
			return points[i].Y < points[j].Y
		}
		return points[i].X < points[j].X
	})

	// Build lower hull
	var lowerHull []Vector2D
	for _, p := range points {
		for len(lowerHull) >= 2 && !isClockwiseTurn(lowerHull[len(lowerHull)-2], lowerHull[len(lowerHull)-1], p) {
			lowerHull = lowerHull[:len(lowerHull)-1]
		}
		lowerHull = append(lowerHull, p)
	}

	// Build upper hull
	var upperHull []Vector2D
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		for len(upperHull) >= 2 && !isClockwiseTurn(upperHull[len(upperHull)-2], upperHull[len(upperHull)-1], p) {
			upperHull = upperHull[:len(upperHull)-1]
		}
		upperHull = append(upperHull, p)
	}

	// Combine lower and upper hulls to form the convex hull
	convexHullPoints := append(lowerHull[:len(lowerHull)-1], upperHull[:len(upperHull)-1]...)
	return convexHullPoints
}

// isClockwiseTurn checks if the three points make a clockwise turn
func isClockwiseTurn(a, b, c Vector2D) bool {
	return (b.Y-a.Y)*(c.X-b.X) > (b.X-a.X)*(c.Y-b.Y)
}

const (
	substeps = 8
	dt       = 1.0 / substeps
	fps      = 60
)

func CollisionBallLine(ball *Ball, line Line) {
	Line1 := Vector2D{
		X: line.To.X - line.From.X,
		Y: line.To.Y - line.From.Y,
	}

	Line2 := Vector2D{
		X: ball.Position.X - line.From.X,
		Y: ball.Position.Y - line.From.Y,
	}

	LineLength := math.Pow(Line1.X, 2) + math.Pow(Line1.Y, 2)

	t := max(0, min(1, Dot(Line1, Line2)/LineLength))

	Point := Vector2D{
		X: line.From.X + t*Line1.X,
		Y: line.From.Y + t*Line1.Y,
	}

	dist := math.Sqrt(math.Pow(Point.X-ball.Position.X, 2) + math.Pow(Point.Y-ball.Position.Y, 2))

	if dist < ball.Radius {
		overlap := ball.Radius - dist

		radians := math.Atan2(Point.Y-ball.Position.Y, Point.X-ball.Position.X)

		ball.Position.X -= math.Cos(radians) * overlap
		ball.Position.Y -= math.Sin(radians) * overlap

		radians = math.Atan2(Point.Y-ball.Position.Y, Point.X-ball.Position.X)

		angle := math.Round(math.Abs(radians * 180 / math.Pi))

		if angle == 90 || angle == 270 {
			ball.Velocity.Y *= -1
		} else {
			ball.Velocity.X *= -1
		}
	}
}

func CollisionBallBall(ball1, ball2 *Ball) {
	distance := math.Sqrt(math.Pow(ball1.Position.X-ball2.Position.X, 2) + math.Pow(ball1.Position.Y-ball2.Position.Y, 2))
	angle := math.Atan2(ball1.Position.Y-ball2.Position.Y, ball1.Position.X-ball2.Position.X)

	overlap := ball1.Radius + ball2.Radius - distance
	impulse := math.Sqrt(math.Pow(ball1.Velocity.X, 2) + math.Pow(ball1.Velocity.Y, 2))
	impulse2 := math.Sqrt(math.Pow(ball2.Velocity.X, 2) + math.Pow(ball2.Velocity.Y, 2))

	if overlap > 0 {
		ball1.Position.X = ball1.Position.X + math.Cos(angle)*(overlap/2)
		ball1.Position.Y = ball1.Position.Y + math.Sin(angle)*(overlap/2)
		ball2.Position.X = ball2.Position.X - math.Cos(angle)*(overlap/2)
		ball2.Position.Y = ball2.Position.Y - math.Sin(angle)*(overlap/2)

		ball1.Velocity.X = math.Cos(angle) * impulse
		ball1.Velocity.Y = math.Sin(angle) * impulse
		ball2.Velocity.X = -math.Cos(angle) * impulse2
		ball2.Velocity.Y = -math.Sin(angle) * impulse2
	}
}

func Dot(a, b Vector2D) float64 {
	return a.X*b.X + a.Y*b.Y
}

func Simulate() {
	step := 0

	for {
		step++

		for st := 0; st < substeps; st++ {
			for i, ball := range balls {
				// gravity
				ball.Velocity.Y += 2 * dt
				ball.Velocity.X *= 0.999
				ball.Velocity.Y *= 0.999

				ball.Position.X += ball.Velocity.X * dt
				ball.Position.Y += ball.Velocity.Y * dt

				for _, line := range polygon.Lines {
					CollisionBallLine(&ball, line)
				}

				for j, otherBall := range balls {
					if i == j {
						continue
					}

					CollisionBallBall(&ball, &otherBall)
				}

				balls[i] = ball
			}
		}

		time.Sleep(40 * time.Millisecond)
		EmitBalls()
	}
}

func UpdatePolygon() {
	polygon = Polygon{}

	for _, client := range clients {

		if client.window == nil {
			continue
		}


		polygon = MergePolygons(polygon, WindowToPolygon(*client.window))
	}

	polygon.Points = ComputeOrthogonalConvexHull(polygon.Points)
	// polygon.Lines = append(polygon.Lines, Line{
	//   From: Vector2D{
	//     X: 300,
	//     Y: 300,
	//   },
	//   To: Vector2D{
	//     X: 600,
	//     Y: 400,
	//   },
	// })

	for i := 0; i < len(polygon.Points); i++ {
		current := polygon.Points[i]
		next := polygon.Points[(i+1)%len(polygon.Points)]

		vertical_distance := math.Abs(next.Y - current.Y)
		horizontal_distance := math.Abs(next.X - current.X)

		if vertical_distance < horizontal_distance {
			polygon.Lines = append(polygon.Lines, Line{
				From: current,
				To:   Vector2D{X: next.X, Y: current.Y},
			}, Line{
				From: Vector2D{X: next.X, Y: current.Y},
				To:   next,
			})
		} else {
			polygon.Lines = append(polygon.Lines, Line{
				From: current,
				To:   Vector2D{X: current.X, Y: next.Y},
			}, Line{
				From: Vector2D{X: current.X, Y: next.Y},
				To:   next,
			})
		}
	}

	for _, client := range clients {
		client.conn.WriteJSON(struct {
			Type string  `json:"type"`
			Data Polygon `json:"data"`
		}{
			Type: "polygon",
			Data: polygon,
		})
	}
}

func main() {
	app = gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"} // Substitua pelo seu domínio Svelte

	app.Use(cors.New(config))

	app.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Print("upgrade:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		defer conn.Close()

		for {
			_, p, err := conn.ReadMessage()

			var event Event
			var ballEvent BallEvent
			message := string(p)

			json.Unmarshal([]byte(message), &event)
			json.Unmarshal([]byte(message), &ballEvent)

			if err != nil {
        delete(clients, event.Data.ID)
				return
			}

			switch event.Type {
			case "new-window":
				NewWindow(conn, event)
				UpdatePolygon()
			case "update-window":
				UpdateWindow(conn, event)
				UpdatePolygon()
			case "close-window":
				delete(clients, event.Data.ID)
				UpdatePolygon()
			case "new-ball":
				NewBall(conn, ballEvent)
			}
		}
	})

	app.Use(static.Serve("/", static.LocalFile("public", false)))

	go Simulate()

	app.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func NewBall(conn *websocket.Conn, event BallEvent) {
	ball := event.Data
	balls = append(balls, ball)
}

func NewWindow(conn *websocket.Conn, event Event) {
	window := event.Data
	window.ID = currentClientId

	clients[currentClientId] = Client{
		id:     currentClientId,
		conn:   conn,
		window: &window,
	}

	conn.WriteJSON(struct {
		Type string `json:"type"`
		Data Window `json:"data"`
	}{
		Type: "new-window",
		Data: window,
	})

	currentClientId++
}

func UpdateWindow(conn *websocket.Conn, event Event) {
	window := event.Data

	clients[window.ID] = Client{
		id:     window.ID,
		conn:   conn,
		window: &window,
	}

	Emit(event)
}

func CloseWindow(conn *websocket.Conn, event Event) {
	window := event.Data

	delete(clients, window.ID)
}

func EmitBalls() {
	data := struct {
		Type string `json:"type"`
		Data []Ball `json:"data"`
	}{
		Type: "balls",
		Data: balls,
	}

	for _, client := range clients {
		err := client.conn.WriteJSON(data)
		if err != nil {
			delete(clients, client.id)
		}
	}
}

func Emit(event Event) {
	for _, client := range clients {
    err := client.conn.WriteJSON(event)
    if err != nil {
      delete(clients, client.id)
    }
	}
}

func remove(windows []Window, s int) []Window {
	for i, window := range windows {
		if window.ID == s {
			return append(windows[:i], windows[i+1:]...)
		}
	}
	return windows
}

func removeLines(lines []Line, s int) []Line {
	return append(lines[:s], lines[s+1:]...)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	app.ServeHTTP(w, r)
}
