let lander;
let terrain = [];
let fuel = 100;
let gravity = 0.02;;
let thrust = 0.05;
let landed = false;
let crashed = false;
let padStart, padEnd, padY;
let level = 1;
let stars = [];
let starTimer = 0;
let maxStars = 0;

function setup() {
	c = createCanvas(window.innerWidth, window.innerHeight);
	document.getElementById("canvascontainer").appendChild(c.canvas);
	document.body.scrollTop = 0;
	document.body.style.overflow = 'hidden';
	initGame();
}

function initGame() {
	lander = {
		x: width / 2,
		y: 50,
		vx: 0,
		vy: 0,
		size: 15
	};

	fuel = 100;
	landed = false;
	crashed = false;
	terrain = [];
	stars = [];
	starTimer = 0;

	// Generate terrain with increasing difficulty
	let xoff = 0;
	let roughness = 100 + level * 20; // terrain gets rougher with level
	for (let x = 0; x < width; x += 10) {
		terrain.push(height - noise(xoff) * roughness - 100);
		xoff += 0.1;
	}

	// Choose landing pad
	padStart = floor(random(10, terrain.length - 10));
	padEnd = padStart + 5;
	padY = terrain[padStart];
	for (let i = padStart; i <= padEnd; i++) {
		terrain[i] = padY; // flatten pad
	}

	// Set maximum stars for this level
	maxStars = 2 + level;
}

function newStar() {
	let fromLeft = random() < 0.5;
	let speedRange = 2 + level * 0.5; // stars get faster with level
	let x = fromLeft ? 0 : width;

	// Pick y above terrain at this x
	let terrainIndex = constrain(floor(x / 10), 0, terrain.length - 1);
	let minY = terrain[terrainIndex] - 50; // at least 50px above ground
	let y = random(50, minY);

	return {
		x: x,
		y: y,
		vx: fromLeft ? random(2, 2 + speedRange) : random(-2 - speedRange, -2),
		vy: random(-0.5, 0.5), // small vertical movement
		size: random(5, 12)
	};
}

function draw() {
	background(0);
	stroke(255);
	noFill();

	// Draw terrain
	beginShape();
	for (let x = 0; x < terrain.length; x++) {
		if (x === padStart) {
			endShape();
			stroke(0, 255, 0); // pad in green
			beginShape();
		}
		if (x === padEnd + 1) {
			endShape();
			stroke(255); // back to white
			beginShape();
		}
		vertex(x * 10, terrain[x]);
	}
	endShape();

	if (!landed && !crashed) {
		// Apply gravity
		lander.vy += gravity;

		// Controls
		if (keyIsDown(UP_ARROW) && fuel > 0) {
			lander.vy -= thrust;
			fuel -= 0.2;
		}
		if (keyIsDown(LEFT_ARROW) && fuel > 0) {
			lander.vx -= thrust;
			fuel -= 0.1;
		}
		if (keyIsDown(RIGHT_ARROW) && fuel > 0) {
			lander.vx += thrust;
			fuel -= 0.1;
		}

		// Update position
		lander.x += lander.vx;
		lander.y += lander.vy;

		// keep in bounds horizontally
		if (lander.x < 0) {
			lander.x = 0
			lander.vx = 0
		} else if (lander.x > window.innerWidth) {
			lander.x = window.innerWidth
			lander.vx = 0
		}
		// simple ceiling
		if (lander.y < 10) {
			lander.y = 10
			lander.vy = 0
		}

		// Collision detection with terrain
		let tx = floor(lander.x / 10);
		if (tx >= 0 && tx < terrain.length) {
			if (lander.y + lander.size / 2 > terrain[tx]) {
				if (
					tx >= padStart &&
					tx <= padEnd &&
					abs(lander.vy) < 1 &&
					abs(lander.vx) < 1
				) {
					landed = true;
				} else {
					crashed = true;
				}
			}
		}

		// Gradual star spawning
		if (stars.length < maxStars) {
			starTimer++;
			let spawnRate = max(120 - level * 10, 30); // faster spawn at higher levels
			if (starTimer > spawnRate) {
				stars.push(newStar());
				starTimer = 0;
			}
		}

		// Shooting stars update
		for (let star of stars) {
			star.x += star.vx;
			star.y += star.vy;

			// Bounce + damage terrain
			let tx = constrain(floor(star.x / 10), 0, terrain.length - 1);
			if (star.y > terrain[tx] - 5) {
				if (tx + 1 >= terrain.length) {
					tx = tx - 1;
				}
				let y2 = terrain[tx + 1];
				let y1 = terrain[tx];
				let x2 = tx + 10;
				let x1 = tx;
				if (star.x < tx) {
					y2 = terrain[tx];
					y1 = terrain[tx - 1];
					x2 = tx;
					x1 = tx - 10;
				}
				let nx = (-1 * (y2 - y1)) / Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2);
				let ny = (x2 - x1) / Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2);
				let b = random(0.5, 1.0); // b=0.0 -> no bounce, b=1.0 -> no loss of speed
				let dotpr = nx * star.vx + ny * star.vy;
				star.vy = b * ((-2 * dotpr) * ny + star.vy)
				star.vx = b * ((-2 * dotpr) * nx + star.vx)

				star.y = terrain[tx] - 5;

				// Damage terrain -> carve crater
				for (let i = -2; i <= 2; i++) {
					let idx = tx + i;
					if (idx >= 0 && idx < terrain.length) {
						terrain[idx] += 10; // lower terrain
					}
				}
			}

			// Respawn if out of bounds
			if (star.x < -20 || star.x > width + 20) {
				Object.assign(star, newStar());
			}

			// Collision with lander
			let d = dist(lander.x, lander.y, star.x, star.y);
			if (d < lander.size / 2 + star.size / 2) {
				crashed = true;
			}
		}
	}

	// Draw lander
	push();
	translate(lander.x, lander.y);
	stroke(255);
	fill(0);
	triangle(
		-lander.size / 2,
		lander.size / 2,
		lander.size / 2,
		lander.size / 2,
		0,
		-lander.size / 2
	);

	// Thruster flames animation
	stroke(255, 0, 0);
	if (keyIsDown(UP_ARROW) && fuel > 0) {
		line(-3, lander.size / 2, 0, lander.size / 2 + random(5, 10));
		line(3, lander.size / 2, 0, lander.size / 2 + random(5, 10));
	}
	if (keyIsDown(LEFT_ARROW) && fuel > 0) {
		line(lander.size / 2, 0, lander.size / 2 + random(5, 10), 0);
	}
	if (keyIsDown(RIGHT_ARROW) && fuel > 0) {
		line(-lander.size / 2, 0, -lander.size / 2 - random(5, 10), 0);
	}
	pop();

	// Draw stars
	for (let star of stars) {
		stroke(255, 255, 0);
		strokeWeight(star.size);
		point(star.x, star.y);

		// Tail effect
		strokeWeight(1);
		stroke(255, 200, 0, 150);
		line(star.x, star.y, star.x - star.vx * 5, star.y - star.vy * 5);
	}

	// HUD
	noStroke();
	fill(0, 255, 0);
	textSize(12);
	text("LEVEL: " + level, width - 100, 20);
	text("FUEL: " + nf(fuel, 1, 0), 10, 20);
	text("VEL: " + nf(lander.vy, 1, 2), 10, 40);

	if (landed) {
		fill(0, 255, 0);
		text("LANDED SUCCESSFULLY!", width / 2 - 80, height / 2);
		text("Press N for Next Level", width / 2 - 80, height / 2 + 20);
	}
	if (crashed) {
		fill(255, 0, 0);
		text("CRASH!", width / 2 - 20, height / 2);
		text("Press R to Restart", width / 2 - 60, height / 2 + 20);
	}
}

function keyPressed() {
	if (crashed && key === "r") {
		initGame();
	}
	if (landed && key === "n") {
		level++;
		initGame();
	}
}
