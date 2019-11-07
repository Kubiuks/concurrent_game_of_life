package main

import (
	"fmt"
	"strconv"
	"strings"
)

const dead = 0x00
const live  = 0xFF

func getNumberOfNeighbors(world [][]byte, y,x,height,width int) int {
	neighbors := 0
	for i:=0; i<3;i++{
		for j:=0; j<3; j++{

			if world[(y-1+i+height)%height][(x-1+j+width)%width] == 0xFF{
				neighbors++
			}
		}
	}
	// we iterate through the main cell as well so we need to subtract it from the count
	if world[y][x] == live {
		neighbors--
	}
	return neighbors
}


// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell) {

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)
	newWorld := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
		newWorld[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
				newWorld[y][x] = val
			}
		}
	}

	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				// Placeholder for the actual Game of Life logic

				//we create a world for the next round in a new matrix
				//so the intermediate results dont affect current world

				//counting the number of alive neighbors
				neighbors := getNumberOfNeighbors(world, y ,x, p.imageHeight, p.imageWidth)

				if neighbors < 2 || neighbors > 3 {
					newWorld[y][x] = dead
				} else if neighbors == 3 {
					newWorld[y][x] = live
				}
			}
		}
		//updating the world to the new state
		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
			world[y][x] = newWorld[y][x]
			}
		}
	}

	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}

	//writing the pgm file
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outVal <- world[y][x]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}
