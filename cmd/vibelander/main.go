package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenW      = 1280
	ScreenH      = 860
	SegW         = 10 // terrain horizontal resolution
	MaxLives     = 3
	Gravity      = 0.02
	MainThrust   = 0.05
	SideThrust   = 0.03
	InitialFuel  = 100.0
	CraterDepth  = 10.0 // how much terrain y increases on damage (larger y is lower)
	SpawnMinYGap = 50   // spawn at least this much above terrain
)

var (
	textFont            font.Face
	textFontGoXFace     *text.GoXFace
	hudTextGeoMatrix    ebiten.GeoM
	landedTextGeoMatrix ebiten.GeoM
	crashTextGeoMatrix  ebiten.GeoM

	hudTextColorScale    = ebiten.ColorScale{}
	landedTextColorScale = ebiten.ColorScale{}
	crashTextColorScale  = ebiten.ColorScale{}

	hudTextDrawOptions    *text.DrawOptions
	landedTextDrawOptions *text.DrawOptions
	crashTextDrawOptions  *text.DrawOptions
)

type Lander struct {
	x, y   float64
	vx, vy float64
	size   float64
}

type Star struct {
	x, y   float64
	vx, vy float64
	size   float64
}

type Game struct {
	lander    Lander
	lives     int
	substract int
	terrain   []float64
	padStart  int
	padEnd    int
	padY      float64

	level    int
	fuel     float64
	landed   bool
	crashed  bool
	gameover bool

	stars     []*Star
	starTimer int
	maxStars  int

	rng *rand.Rand
}

func NewGame() *Game {
	// set fonts
	tt, _ := opentype.Parse(fonts.ArcadeN_ttf)
	textFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    float64(10),
		DPI:     72.0,
		Hinting: font.HintingVertical,
	})
	textFontGoXFace = text.NewGoXFace(textFont)

	hudTextGeoMatrix = ebiten.GeoM{}
	hudTextColorScale.Scale(0, 1.0, 0, 1.0)
	hudTextDrawOptions = &text.DrawOptions{
		DrawImageOptions: ebiten.DrawImageOptions{
			GeoM:       hudTextGeoMatrix,
			ColorScale: hudTextColorScale,
		},
	}

	landedTextGeoMatrix = ebiten.GeoM{}
	landedTextColorScale.Scale(0, 1.0, 0, 1.0)
	landedTextDrawOptions = &text.DrawOptions{
		DrawImageOptions: ebiten.DrawImageOptions{
			GeoM:       landedTextGeoMatrix,
			ColorScale: landedTextColorScale,
		},
	}

	crashTextGeoMatrix = ebiten.GeoM{}
	crashTextColorScale.Scale(1.0, 0, 0, 1.0)
	crashTextDrawOptions = &text.DrawOptions{
		DrawImageOptions: ebiten.DrawImageOptions{
			GeoM:       crashTextGeoMatrix,
			ColorScale: crashTextColorScale,
		},
	}

	g := &Game{
		level:    1,
		lives:    MaxLives,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
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
	g.substract = 1

	// create terrain
	nSeg := ScreenW / SegW
	g.terrain = make([]float64, nSeg)
	amp := 20.0 + float64(g.level)*5.0
	seed := g.rng.Float64() * 10.0
	for i := range nSeg {
		// combination of sine waves + small randomness for retro look
		base := float64(ScreenH) - 200.0
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
	if !g.landed && !g.crashed && !g.gameover {
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
			spawnRate = int(math.Max(float64(spawnRate), 30))
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
				if idx+1 >= len(g.terrain) {
					idx = idx - 1
				}
				y2 := g.terrain[idx+1]
				y1 := g.terrain[idx]
				x2 := idx + 10
				x1 := idx
				if int(st.x) < idx && idx-1 > -1 {
					y2 = g.terrain[idx]
					y1 = g.terrain[idx-1]
					x2 = idx
					x1 = idx - 10
				}
				nx := (-1 * (y2 - y1)) / math.Sqrt(float64(x2-x1)*float64(x2-x1)+float64(y2-y1)*float64(y2-y1))
				ny := float64(x2-x1) / math.Sqrt(float64(x2-x1)*float64(x2-x1)+float64(y2-y1)*float64(y2-y1))
				b := 0.5 + g.rng.Float64()*0.5 // b=0.0 -> no bounce, b=1.0 -> no loss of speed
				dotpr := nx*st.vx + ny*st.vy
				st.vy = b * ((-2*dotpr)*ny + st.vy)
				st.vx = b * ((-2*dotpr)*nx + st.vx)
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
	if ebiten.IsKeyPressed(ebiten.KeyR) && g.gameover {
		g.gameover = false
		g.level = 1
		g.lives = MaxLives
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
		x2 := float64((i + 1) * SegW)
		y2 := g.terrain[i+1]
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, color.White, false)
	}

	// highlight pad in green (draw top line over terrain)
	for i := g.padStart; i <= g.padEnd && i < len(g.terrain)-1; i++ {
		x1 := float64(i * SegW)
		y1 := g.terrain[i]
		x2 := float64((i + 1) * SegW)
		y2 := g.terrain[i+1]
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, color.RGBA{0, 200, 0, 255}, false)
	}

	// draw stars (tail + point)
	for _, st := range g.stars {
		// tail
		tx := st.x - st.vx*5
		ty := st.y - st.vy*5
		vector.StrokeLine(screen, float32(st.x), float32(st.y), float32(tx), float32(ty), 1, color.RGBA{255, 200, 0, 200}, false)
		// point (draw small cross or filled-like)
		sz := st.size
		vector.StrokeLine(screen, float32(st.x-sz/2), float32(st.y), float32(st.x+sz/2), float32(st.y), 1, color.RGBA{255, 255, 0, 255}, false)
		vector.StrokeLine(screen, float32(st.x), float32(st.y-sz/2), float32(st.x), float32(st.y+sz/2), 1, color.RGBA{255, 255, 0, 255}, false)
	}

	// draw lander (retro triangle wireframe)
	px := g.lander.x
	py := g.lander.y
	s := g.lander.size
	// bottom left, bottom right, top
	x1, y1 := px-s/2, py+s/2
	x2, y2 := px+s/2, py+s/2
	x3, y3 := px, py-s/2
	vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, color.White, false)
	vector.StrokeLine(screen, float32(x2), float32(y2), float32(x3), float32(y3), 1, color.White, false)
	vector.StrokeLine(screen, float32(x3), float32(y3), float32(x1), float32(y1), 1, color.White, false)

	// thruster flames when keys pressed
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) && g.fuel > 0 && !g.landed && !g.crashed {
		// two flame lines flicker
		vector.StrokeLine(screen, float32(px-3), float32(py+s/2), float32(px), float32(py+s/2+5+g.rng.Float64()*6), 1, color.RGBA{255, 80, 0, 255}, false)
		vector.StrokeLine(screen, float32(px+3), float32(py+s/2), float32(px), float32(py+s/2+5+g.rng.Float64()*6), 1, color.RGBA{255, 80, 0, 255}, false)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) && g.fuel > 0 && !g.landed && !g.crashed {
		vector.StrokeLine(screen, float32(px+s/2), float32(py), float32(px+s/2+5+g.rng.Float64()*6), float32(py), 1, color.RGBA{255, 80, 0, 255}, false)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) && g.fuel > 0 && !g.landed && !g.crashed {
		vector.StrokeLine(screen, float32(px-s/2), float32(py), float32(px-s/2-5-g.rng.Float64()*6), float32(py), 1, color.RGBA{255, 80, 0, 255}, false)
	}

	// Status messages
	if g.landed {
		landedTextDrawOptions.GeoM.Translate(ScreenW/2-100, ScreenH/2-50)
		text.Draw(screen, "LANDED SUCCESSFULLY!", textFontGoXFace, landedTextDrawOptions)
		landedTextDrawOptions.GeoM.Translate(0, 20)
		text.Draw(screen, "Press N for Next Level", textFontGoXFace, landedTextDrawOptions)
		landedTextDrawOptions.GeoM.Reset()
	}
	if g.crashed && !g.gameover {
		g.lives -= g.substract
		g.substract = 0
		crashTextDrawOptions.GeoM.Translate(ScreenW/2-100, ScreenH/2-50)
		text.Draw(screen, "CRASH!", textFontGoXFace, crashTextDrawOptions)
		crashTextDrawOptions.GeoM.Translate(0, 20)
		text.Draw(screen, "Press R to Restart level", textFontGoXFace, crashTextDrawOptions)
		crashTextDrawOptions.GeoM.Reset()
	}

	// HUD
	hudTextDrawOptions.GeoM.Translate(ScreenW-100, 20)
	text.Draw(screen, fmt.Sprintf("LEVEL: %d", g.level), textFontGoXFace, hudTextDrawOptions)
	hudTextDrawOptions.GeoM.Translate(0, 20)
	text.Draw(screen, fmt.Sprintf("FUEL: %d", int(g.fuel+0.5)), textFontGoXFace, hudTextDrawOptions)
	hudTextDrawOptions.GeoM.Translate(0, 20)
	text.Draw(screen, fmt.Sprintf("VEL: %.2f", g.lander.vy), textFontGoXFace, hudTextDrawOptions)
	hudTextDrawOptions.GeoM.Translate(0, 20)
	text.Draw(screen, fmt.Sprintf("LIVES:%s", strings.Repeat("|", g.lives)), textFontGoXFace, hudTextDrawOptions)
	hudTextDrawOptions.GeoM.Reset()

	if g.lives == 0 {
		g.gameover = true
		crashTextDrawOptions.GeoM.Translate(ScreenW/2-100, ScreenH/2-50)
		text.Draw(screen, "GAMEOVER!", textFontGoXFace, crashTextDrawOptions)
		crashTextDrawOptions.GeoM.Translate(0, 20)
		text.Draw(screen, "Press R to Restart game", textFontGoXFace, crashTextDrawOptions)
		crashTextDrawOptions.GeoM.Reset()
	}
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
