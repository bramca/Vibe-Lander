// main.go
package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"golang.org/x/image/font/basicfont"
	"github.com/hajimehoshi/ebiten/v2/text"
)

const (
	ScreenW      = 1280
	ScreenH      = 860
	SegW         = 10                 // terrain horizontal resolution
	Gravity      = 0.02
	MainThrust   = 0.05
	SideThrust   = 0.03
	InitialFuel  = 100.0
	CraterDepth  = 10.0               // how much terrain y increases on damage (larger y is lower)
	SpawnMinYGap = 50                 // spawn at least this much above terrain
)

type Lander struct {
	x, y float64
	vx, vy float64
	size float64
}

type Star struct {
	x, y float64
	vx, vy float64
	size float64
}

type Game struct {
	lander   Lander
	terrain  []float64
	padStart int
	padEnd   int
	padY     float64

	level    int
	fuel     float64
	landed   bool
	crashed  bool

	stars    []*Star
	starTimer int
	maxStars int

	rng *rand.Rand
}

func NewGame() *Game {
	g := &Game{
		level: 1,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	g.initGame()
	return g
}

func (g *Game) initGame() {
	// initialize lander
	g.lander = Lander{
		x:    ScreenW / 2,
		y:    50,
		vx:   0,
		vy:   0,
		size: 15,
	}
	g.fuel = InitialFuel
	g.landed = false
	g.crashed = false

	// create terrain
	nSeg := ScreenW / SegW
	g.terrain = make([]float64, nSeg)
	amp := 50.0 + float64(g.level)*10.0
	seed := g.rng.Float64() * 10.0
	for i := 0; i < nSeg; i++ {
		// combination of sine waves + small randomness for retro look
		base := float64(ScreenH) - 100.0
		h := base - math.Sin(float64(i)*0.25+seed)*amp - math.Cos(float64(i)*0.13+seed*1.5)*(amp*0.4) + g.rng.Float64()*amp*0.3
		g.terrain[i] = h
	}

	// choose landing pad and flatten it
	g.padStart = g.rng.Intn(len(g.terrain)-12) + 6
	g.padEnd = g.padStart + 5
	g.padY = g.terrain[g.padStart]
	for i := g.padStart; i <= g.padEnd && i < len(g.terrain); i++ {
		g.terrain[i] = g.padY
	}

	// stars start gradually; set max stars based on level
	g.stars = []*Star{}
	g.starTimer = 0
	g.maxStars = 2 + g.level // start small, increase with level
}

func (g *Game) spawnStar() {
	fromLeft := g.rng.Float64() < 0.5
	var x float64
	if fromLeft {
		x = 0
	} else {
		x = ScreenW
	}

	idx := int(x) / SegW
	if idx < 0 {
		idx = 0
	} else if idx >= len(g.terrain) {
		idx = len(g.terrain) - 1
	}
	minY := g.terrain[idx] - SpawnMinYGap
	if minY < 60 {
		minY = 60
	}
	y := g.rng.Float64()*(minY-50.0) + 50.0

	speedRange := 2.0 + float64(g.level)*0.5
	var vx float64
	if fromLeft {
		vx = g.rng.Float64()*(2.0+speedRange-2.0) + 2.0
	} else {
		vx = -(g.rng.Float64()*(2.0+speedRange-2.0) + 2.0)
	}
	vy := g.rng.Float64()*1.0 - 0.5
	size := g.rng.Float64()*7.0 + 5.0

	s := &Star{x: x, y: y, vx: vx, vy: vy, size: size}
	g.stars = append(g.stars, s)
}

func (g *Game) Update() error {
	// Input: if crashed show R restart; if landed show N next level handled in KeyPressed
	// Physics updates only when not landed/crashed
	if !g.landed && !g.crashed {
		// gravity
		g.lander.vy += Gravity

		// controls
		if ebiten.IsKeyPressed(ebiten.KeyArrowUp) && g.fuel > 0 {
			g.lander.vy -= MainThrust
			g.fuel -= 0.2
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) && g.fuel > 0 {
			g.lander.vx -= SideThrust
			g.fuel -= 0.1
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowRight) && g.fuel > 0 {
			g.lander.vx += SideThrust
			g.fuel -= 0.1
		}

		// update pos
		g.lander.x += g.lander.vx
		g.lander.y += g.lander.vy

		// keep in bounds horizontally
		if g.lander.x < 0 {
			g.lander.x = 0
			g.lander.vx = 0
		} else if g.lander.x > ScreenW {
			g.lander.x = ScreenW
			g.lander.vx = 0
		}
		// simple ceiling
		if g.lander.y < 10 {
			g.lander.y = 10
			g.lander.vy = 0
		}

		// collision with terrain
		tx := int(g.lander.x) / SegW
		if tx >= 0 && tx < len(g.terrain) {
			if g.lander.y+g.lander.size/2 > g.terrain[tx] {
				// check for soft landing on pad
				if tx >= g.padStart && tx <= g.padEnd && math.Abs(g.lander.vy) < 1.0 && math.Abs(g.lander.vx) < 1.0 {
					g.landed = true
					// snap to pad
					g.lander.y = g.terrain[tx] - g.lander.size/2
					g.lander.vx = 0
					g.lander.vy = 0
				} else {
					g.crashed = true
				}
			}
		}

		// gradual star spawning
		if len(g.stars) < g.maxStars {
			g.starTimer++
			spawnRate := 120 - g.level*10
			if spawnRate < 30 {
				spawnRate = 30
			}
			if g.starTimer > spawnRate {
				g.spawnStar()
				g.starTimer = 0
			}
		}

		// update stars
		for _, st := range g.stars {
			st.x += st.vx
			st.y += st.vy

			// bounce + damage terrain
			idx := int(st.x) / SegW
			if idx < 0 {
				idx = 0
			} else if idx >= len(g.terrain) {
				idx = len(g.terrain) - 1
			}
			if st.y > g.terrain[idx]-5 {
				// bounce
				y2 := g.terrain[tx + 1];
				y1 := g.terrain[tx];
				x2 := tx + 10;
				x1 := tx;
				if (st.x < tx) {
					y2 = g.terrain[tx];
					y1 = g.terrain[tx - 1];
					x2 = tx;
					x1 = tx - 10;
				}
				nx := (-1 * (y2 - y1)) / Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2);
				ny := (x2 - x1) / Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2);
				b := random(0.5, 1.0); // b=0.0 -> no bounce, b=1.0 -> no loss of speed
				dotpr := nx * st.vx + ny * st.vy;
				st.vy = b * ((-2 * dotpr) * ny + st.vy)
				st.vx = b * ((-2 * dotpr) * nx + st.vx)
				st.vy *= -1
				st.y = g.terrain[idx] - 5

				// damage terrain (crater)
				for di := -2; di <= 2; di++ {
					i := idx + di
					if i >= 0 && i < len(g.terrain) {
						g.terrain[i] += CraterDepth
					}
				}
			}

			// respawn if out of bounds
			if st.x < -40 || st.x > ScreenW+40 {
				// reinitialize star
				*st = *g.newStarObject()
			}

			// collision with lander
			dx := st.x - g.lander.x
			dy := st.y - g.lander.y
			dist := math.Hypot(dx, dy)
			if dist < (g.lander.size/2 + st.size/2) {
				g.crashed = true
			}
		}
	}

	// global keys handled in KeyPressed in Ebiten style: we check here for R/N immediate actions
	// (Ebiten recommends checking IsKeyPressed in Update; but we want key-down events for R/N — we'll implement a small edge detector)
	// For simplicity, allow pressing R or N anytime (but only trigger when appropriate)
	if ebiten.IsKeyPressed(ebiten.KeyR) && g.crashed {
		g.initGame()
	}
	if ebiten.IsKeyPressed(ebiten.KeyN) && g.landed {
		g.level++
		g.initGame()
	}

	// increase maxStars slightly as level passes (so if level changed)
	g.maxStars = 2 + g.level
	return nil
}

func (g *Game) newStarObject() *Star {
	fromLeft := g.rng.Float64() < 0.5
	var x float64
	if fromLeft {
		x = 0
	} else {
		x = ScreenW
	}
	idx := int(x) / SegW
	if idx < 0 {
		idx = 0
	} else if idx >= len(g.terrain) {
		idx = len(g.terrain) - 1
	}
	minY := g.terrain[idx] - SpawnMinYGap
	if minY < 60 {
		minY = 60
	}
	y := g.rng.Float64()*(minY-50.0) + 50.0

	speedRange := 2.0 + float64(g.level)*0.5
	var vx float64
	if fromLeft {
		vx = g.rng.Float64()*(2.0+speedRange-2.0) + 2.0
	} else {
		vx = -(g.rng.Float64()*(2.0+speedRange-2.0) + 2.0)
	}
	vy := g.rng.Float64()*1.0 - 0.5
	size := g.rng.Float64()*7.0 + 5.0
	return &Star{x: x, y: y, vx: vx, vy: vy, size: size}
}

func (g *Game) Draw(screen *ebiten.Image) {
	// clear
	screen.Fill(color.RGBA{0, 0, 0, 255})

	// draw terrain as polyline
	colslice := len(g.terrain)
	// draw shape by iterating segments
	for i := 0; i < colslice-1; i++ {
		x1 := float64(i * SegW)
		y1 := g.terrain[i]
		x2 := float64((i+1) * SegW)
		y2 := g.terrain[i+1]
		ebitenutil.DrawLine(screen, x1, y1, x2, y2, color.White)
	}

	// highlight pad in green (draw top line over terrain)
	for i := g.padStart; i <= g.padEnd && i < len(g.terrain)-1; i++ {
		x1 := float64(i * SegW)
		y1 := g.terrain[i]
		x2 := float64((i+1) * SegW)
		y2 := g.terrain[i+1]
		ebitenutil.DrawLine(screen, x1, y1, x2, y2, color.RGBA{0, 200, 0, 255})
	}

	// draw stars (tail + point)
	for _, st := range g.stars {
		// tail
		tx := st.x - st.vx*5
		ty := st.y - st.vy*5
		ebitenutil.DrawLine(screen, st.x, st.y, tx, ty, color.RGBA{255, 200, 0, 200})
		// point (draw small cross or filled-like)
		sz := st.size
		ebitenutil.DrawLine(screen, st.x-sz/2, st.y, st.x+sz/2, st.y, color.RGBA{255, 255, 0, 255})
		ebitenutil.DrawLine(screen, st.x, st.y-sz/2, st.x, st.y+sz/2, color.RGBA{255, 255, 0, 255})
	}

	// draw lander (retro triangle wireframe)
	px := g.lander.x
	py := g.lander.y
	s := g.lander.size
	// bottom left, bottom right, top
	x1, y1 := px-s/2, py+s/2
	x2, y2 := px+s/2, py+s/2
	x3, y3 := px, py-s/2
	ebitenutil.DrawLine(screen, x1, y1, x2, y2, color.White)
	ebitenutil.DrawLine(screen, x2, y2, x3, y3, color.White)
	ebitenutil.DrawLine(screen, x3, y3, x1, y1, color.White)

	// thruster flames when keys pressed
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) && g.fuel > 0 && !g.landed && !g.crashed {
		// two flame lines flicker
		ebitenutil.DrawLine(screen, px-3, py+s/2, px, py+s/2+5+g.rng.Float64()*6, color.RGBA{255, 80, 0, 255})
		ebitenutil.DrawLine(screen, px+3, py+s/2, px, py+s/2+5+g.rng.Float64()*6, color.RGBA{255, 80, 0, 255})
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) && g.fuel > 0 && !g.landed && !g.crashed {
		ebitenutil.DrawLine(screen, px+s/2, py, px+s/2+5+g.rng.Float64()*6, py, color.RGBA{255, 80, 0, 255})
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) && g.fuel > 0 && !g.landed && !g.crashed {
		ebitenutil.DrawLine(screen, px-s/2, py, px-s/2-5-g.rng.Float64()*6, py, color.RGBA{255, 80, 0, 255})
	}

	// HUD
	txtCol := color.RGBA{0, 255, 0, 255}
	text.Draw(screen, fmt.Sprintf("LEVEL: %d", g.level), basicfont.Face7x13, ScreenW-100, 20, txtCol)
	text.Draw(screen, fmt.Sprintf("FUEL: %d", int(g.fuel+0.5)), basicfont.Face7x13, 10, 20, txtCol)
	text.Draw(screen, fmt.Sprintf("VEL: %.2f", g.lander.vy), basicfont.Face7x13, 10, 40, txtCol)

	// Status messages
	if g.landed {
		text.Draw(screen, "LANDED SUCCESSFULLY!", basicfont.Face7x13, ScreenW/2-100, ScreenH/2, color.RGBA{0, 255, 0, 255})
		text.Draw(screen, "Press N for Next Level", basicfont.Face7x13, ScreenW/2-100, ScreenH/2+20, color.RGBA{0, 255, 0, 255})
	}
	if g.crashed {
		text.Draw(screen, "CRASH!", basicfont.Face7x13, ScreenW/2-20, ScreenH/2, color.RGBA{255, 0, 0, 255})
		text.Draw(screen, "Press R to Restart", basicfont.Face7x13, ScreenW/2-80, ScreenH/2+20, color.RGBA{255, 0, 0, 255})
	}

	// Draw simple instructions top-left (helpful)
	text.Draw(screen, "Controls: Arrow keys to thrust, R restart when crashed, N next level when landed", basicfont.Face7x13, 10, ScreenH-10, color.RGBA{120, 200, 120, 180})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenW, ScreenH
}

func main() {
	ebiten.SetWindowSize(ScreenW, ScreenH)
	ebiten.SetWindowTitle("Vibe Lander")

	g := NewGame()
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
