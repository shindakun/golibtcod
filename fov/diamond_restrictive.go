package fov

// Faithful ports of fov_diamond_raycasting.c and fov_restrictive.c.
// Ported from libtcod, BSD 3-Clause License, © 2008-2026 Jice and the
// libtcod contributors. See LICENSE.txt.

/* --- FOV_DIAMOND: diamond raycasting --- */

type raycastTile struct {
	xRelative, yRelative   int
	xObscurity, yObscurity int
	xError, yError         int
	xInput, yInput         *raycastTile
	perimeterNext          *raycastTile
	touched                bool
	ignore                 bool
}

func (r *raycastTile) lengthSq() int {
	return r.xRelative*r.xRelative + r.yRelative*r.yRelative
}

type diamondFov struct {
	m             *Map
	povX, povY    int
	raymapGrid    []raycastTile
	perimeterLast *raycastTile
}

func (f *diamondFov) getRay(relativeX, relativeY int) *raycastTile {
	x := f.povX + relativeX
	y := f.povY + relativeY
	if !f.m.InBounds(x, y) {
		return nil
	}
	ray := &f.raymapGrid[x+y*f.m.w]
	ray.xRelative = relativeX
	ray.yRelative = relativeY
	return ray
}

func (f *diamondFov) processRay(newRay, inputRay *raycastTile) {
	if newRay == nil {
		return
	}
	if newRay.yRelative == inputRay.yRelative {
		newRay.xInput = inputRay
	} else {
		newRay.yInput = inputRay
	}
	if !newRay.touched {
		f.perimeterLast.perimeterNext = newRay
		f.perimeterLast = newRay
		newRay.touched = true
	}
}

func isObscured(ray *raycastTile) bool {
	return (ray.xError > 0 && ray.xError <= ray.xObscurity) ||
		(ray.yError > 0 && ray.yError <= ray.yObscurity)
}

func processXInput(newRay, xInput *raycastTile) {
	if xInput.xObscurity == 0 && xInput.yObscurity == 0 {
		return
	}
	if xInput.xError > 0 && newRay.xObscurity == 0 {
		newRay.xError = xInput.xError - xInput.yObscurity
		newRay.yError = xInput.yError + xInput.yObscurity
		newRay.xObscurity = xInput.xObscurity
		newRay.yObscurity = xInput.yObscurity
	}
	if xInput.yError <= 0 && xInput.yObscurity > 0 && xInput.xError > 0 {
		newRay.yError = xInput.yError + xInput.yObscurity
		newRay.xError = xInput.xError - xInput.yObscurity
		newRay.xObscurity = xInput.xObscurity
		newRay.yObscurity = xInput.yObscurity
	}
}

func processYInput(newRay, yInput *raycastTile) {
	if yInput.xObscurity == 0 && yInput.yObscurity == 0 {
		return
	}
	if yInput.yError > 0 && newRay.yObscurity == 0 {
		newRay.yError = yInput.yError - yInput.xObscurity
		newRay.xError = yInput.xError + yInput.xObscurity
		newRay.xObscurity = yInput.xObscurity
		newRay.yObscurity = yInput.yObscurity
	}
	if yInput.xError <= 0 && yInput.xObscurity > 0 && yInput.yError > 0 {
		newRay.yError = yInput.yError - yInput.xObscurity
		newRay.xError = yInput.xError + yInput.xObscurity
		newRay.xObscurity = yInput.xObscurity
		newRay.yObscurity = yInput.yObscurity
	}
}

func (f *diamondFov) mergeInput(ray *raycastTile) {
	x := ray.xRelative + f.povX
	y := ray.yRelative + f.povY
	rayIndex := x + y*f.m.w

	if ray.xInput != nil {
		processXInput(ray, ray.xInput)
	}
	if ray.yInput != nil {
		processYInput(ray, ray.yInput)
	}
	if ray.xInput == nil {
		if isObscured(ray.yInput) {
			ray.ignore = true
		}
	} else if ray.yInput == nil {
		if isObscured(ray.xInput) {
			ray.ignore = true
		}
	} else if isObscured(ray.xInput) && isObscured(ray.yInput) {
		ray.ignore = true
	}
	if !ray.ignore && !f.m.cells[rayIndex].transparent {
		ray.xError = abs(ray.xRelative)
		ray.xObscurity = ray.xError
		ray.yError = abs(ray.yRelative)
		ray.yObscurity = ray.yError
	}
}

func (f *diamondFov) expandPerimeterFrom(ray *raycastTile) {
	if ray.ignore {
		return
	}
	if ray.xRelative >= 0 {
		f.processRay(f.getRay(ray.xRelative+1, ray.yRelative), ray)
	}
	if ray.xRelative <= 0 {
		f.processRay(f.getRay(ray.xRelative-1, ray.yRelative), ray)
	}
	if ray.yRelative >= 0 {
		f.processRay(f.getRay(ray.xRelative, ray.yRelative+1), ray)
	}
	if ray.yRelative <= 0 {
		f.processRay(f.getRay(ray.xRelative, ray.yRelative-1), ray)
	}
}

func (m *Map) diamondRaycasting(povX, povY, maxRadius int, lightWalls bool) error {
	radiusSquared := maxRadius * maxRadius
	m.cells[povX+povY*m.w].fov = true

	f := &diamondFov{m: m, povX: povX, povY: povY, raymapGrid: make([]raycastTile, len(m.cells))}

	currentRay := f.getRay(0, 0)
	f.perimeterLast = currentRay
	currentRay.touched = true
	f.expandPerimeterFrom(currentRay)

	for currentRay = currentRay.perimeterNext; currentRay != nil; currentRay = currentRay.perimeterNext {
		if radiusSquared <= 0 || currentRay.lengthSq() <= radiusSquared {
			f.mergeInput(currentRay)
		} else {
			currentRay.ignore = true
		}
		f.expandPerimeterFrom(currentRay)

		if currentRay.ignore {
			continue
		}
		if currentRay.xError > 0 && currentRay.xError <= currentRay.xObscurity {
			continue
		}
		if currentRay.yError > 0 && currentRay.yError <= currentRay.yObscurity {
			continue
		}
		mapX := povX + currentRay.xRelative
		mapY := povY + currentRay.yRelative
		m.cells[mapX+mapY*m.w].fov = true
	}
	if lightWalls {
		m.Postprocess(povX, povY, maxRadius)
	}
	return nil
}

/* --- FOV_RESTRICTIVE: Mingos' Restrictive Precise Angle Shadowcasting v1.2 --- */

// restrictiveQuadrant ports compute_quadrant verbatim, including the C
// quirk in the horizontal-edge octant where the inner loop increments idx
// twice per match (present in upstream; preserved deliberately).
func (m *Map) restrictiveQuadrant(povX, povY, maxRadius int, lightWalls bool, dx, dy int, startAngle, endAngle []float64) {
	/* octant: vertical edge */
	{
		iteration := 1
		done := false
		totalObstacles := 0
		obstaclesInLastLine := 0
		minAngle := 0.0

		y := povY + dy
		if y < 0 || y >= m.h {
			done = true
		}
		for !done {
			slopesPerCell := 1.0 / float64(iteration)
			halfSlopes := slopesPerCell * 0.5
			processedCell := int((minAngle + halfSlopes) / slopesPerCell)
			minX := max(0, povX-iteration)
			maxX := min(m.w-1, povX+iteration)
			done = true
			for x := povX + processedCell*dx; x >= minX && x <= maxX; x += dx {
				c := x + y*m.w
				visible := true
				extended := false
				centreSlope := float64(processedCell) * slopesPerCell
				startSlope := centreSlope - halfSlopes
				endSlope := centreSlope + halfSlopes
				if obstaclesInLastLine > 0 {
					prev := c - m.w*dy
					prevSide := c - m.w*dy - dx
					if (!m.cells[prev].fov || !m.cells[prev].transparent) &&
						(!m.cells[prevSide].fov || !m.cells[prevSide].transparent) {
						visible = false
					} else {
						for idx := 0; idx < obstaclesInLastLine && visible; idx++ {
							if startSlope <= endAngle[idx] && endSlope >= startAngle[idx] {
								if m.cells[c].transparent {
									if centreSlope > startAngle[idx] && centreSlope < endAngle[idx] {
										visible = false
									}
								} else {
									if startSlope >= startAngle[idx] && endSlope <= endAngle[idx] {
										visible = false
									} else {
										startAngle[idx] = min64(startAngle[idx], startSlope)
										endAngle[idx] = max64(endAngle[idx], endSlope)
										extended = true
									}
								}
							}
						}
					}
				}
				if visible {
					done = false
					m.cells[c].fov = true
					if !m.cells[c].transparent {
						if minAngle >= startSlope {
							minAngle = endSlope
							if processedCell == iteration {
								done = true
							}
						} else if !extended {
							startAngle[totalObstacles] = startSlope
							endAngle[totalObstacles] = endSlope
							totalObstacles++
						}
						if !lightWalls {
							m.cells[c].fov = false
						}
					}
				}
				processedCell++
			}
			if iteration == maxRadius {
				done = true
			}
			iteration++
			obstaclesInLastLine = totalObstacles
			y += dy
			if y < 0 || y >= m.h {
				done = true
			}
		}
	}

	/* octant: horizontal edge */
	{
		iteration := 1
		done := false
		totalObstacles := 0
		obstaclesInLastLine := 0
		minAngle := 0.0

		x := povX + dx
		if x < 0 || x >= m.w {
			done = true
		}
		for !done {
			slopesPerCell := 1.0 / float64(iteration)
			halfSlopes := slopesPerCell * 0.5
			processedCell := int((minAngle + halfSlopes) / slopesPerCell)
			minY := max(0, povY-iteration)
			maxY := min(m.h-1, povY+iteration)
			done = true
			for y := povY + processedCell*dy; y >= minY && y <= maxY; y += dy {
				c := x + y*m.w
				visible := true
				extended := false
				centreSlope := float64(processedCell) * slopesPerCell
				startSlope := centreSlope - halfSlopes
				endSlope := centreSlope + halfSlopes
				if obstaclesInLastLine > 0 {
					prev := c - dx
					prevSide := c - m.w*dy - dx
					if (!m.cells[prev].fov || !m.cells[prev].transparent) &&
						(!m.cells[prevSide].fov || !m.cells[prevSide].transparent) {
						visible = false
					} else {
						for idx := 0; idx < obstaclesInLastLine && visible; idx++ {
							if startSlope <= endAngle[idx] && endSlope >= startAngle[idx] {
								if m.cells[c].transparent {
									if centreSlope > startAngle[idx] && centreSlope < endAngle[idx] {
										visible = false
									}
								} else {
									if startSlope >= startAngle[idx] && endSlope <= endAngle[idx] {
										visible = false
									} else {
										startAngle[idx] = min64(startAngle[idx], startSlope)
										endAngle[idx] = max64(endAngle[idx], endSlope)
										extended = true
									}
								}
								idx++ // C quirk: double increment in this octant (upstream behavior)
							}
						}
					}
				}
				if visible {
					done = false
					m.cells[c].fov = true
					if !m.cells[c].transparent {
						if minAngle >= startSlope {
							minAngle = endSlope
							if processedCell == iteration {
								done = true
							}
						} else if !extended {
							startAngle[totalObstacles] = startSlope
							endAngle[totalObstacles] = endSlope
							totalObstacles++
						}
						if !lightWalls {
							m.cells[c].fov = false
						}
					}
				}
				processedCell++
			}
			if iteration == maxRadius {
				done = true
			}
			iteration++
			obstaclesInLastLine = totalObstacles
			x += dx
			if x < 0 || x >= m.w {
				done = true
			}
		}
	}
}

func (m *Map) restrictiveShadowcasting(povX, povY, maxRadius int, lightWalls bool) error {
	m.cells[povX+povY*m.w].fov = true
	maxObstacles := max(len(m.cells)/7, 16)
	startAngle := make([]float64, maxObstacles)
	endAngle := make([]float64, maxObstacles)
	m.restrictiveQuadrant(povX, povY, maxRadius, lightWalls, 1, 1, startAngle, endAngle)
	m.restrictiveQuadrant(povX, povY, maxRadius, lightWalls, 1, -1, startAngle, endAngle)
	m.restrictiveQuadrant(povX, povY, maxRadius, lightWalls, -1, 1, startAngle, endAngle)
	m.restrictiveQuadrant(povX, povY, maxRadius, lightWalls, -1, -1, startAngle, endAngle)
	return nil
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
