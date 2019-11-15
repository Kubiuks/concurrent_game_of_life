package main

import (
	"strconv"
	"strings"
)


const dead = 0x00
const live  = 0xFF


func createMatrix(height, width int) [][]byte{
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

//sending the appropriate data to workers
func splitSend(heightOfSmallerArray, width, height, y, counter int, workersIn []chan byte, world [][]byte) {

	// sends the chunk we need to perform the logic on and
	// 1 more layer at the top and 1 more at the bottom(so the logic is correct)
	for h:=y-1; h<heightOfSmallerArray+1+y; h++ {
		for x:=0; x<width; x++ {
			var t int
			if h<0 {
				//top level
				t=(h+height)%height
			} else if h>=height {
				//bottom level
				t=(h-height)%height
			} else {
				//middle level
				t=h
			}
			workersIn[counter]<-world[t][x]
		}
	}
}

//splits the image into parts so workers can work on different parts in parallel
func split(height, heightOfSmallerArray, width, thread int, workersIn, workersOut []chan byte, world [][]byte){

	counter:=0

	//splitting the process of sending the data to different workers
	for y:=0; y<height; y=y+heightOfSmallerArray {
		go splitSend(heightOfSmallerArray, width, height, y, counter, workersIn, world)
		counter++
	}
}

//performs the logic of the game on the given chunk
func worker (channelin, channelout chan byte, heightOfSmallerArray, width int){
	heightOfSmallerArrayWithExtraRows:= heightOfSmallerArray+2


	world:=createMatrix(heightOfSmallerArrayWithExtraRows, width)

	//we create a world for the next round in a new matrix
	//so the intermediate results dont affect current world
	//as this would change the output
	newWorld:=createMatrix(heightOfSmallerArrayWithExtraRows, width)

	for {
		//initialise world and new world in current state
		for y := 0; y < heightOfSmallerArrayWithExtraRows; y++ {
			for x := 0; x < width; x++ {
				val := <-channelin
				world[y][x] = val
				newWorld[y][x] = val
			}
		}
		//
		for y := 1; y < heightOfSmallerArrayWithExtraRows-1; y++ {
			for x := 0; x < width; x++ {
				//counting the number of alive neighbors
				neighbors := getNumberOfNeighbors(world, y, x, heightOfSmallerArrayWithExtraRows, width)
				//performing the actual logic of the game of life
				//and updating the world to the new state
				if neighbors < 2 || neighbors > 3 {
					newWorld[y][x] = dead
				} else if neighbors == 3 {
					newWorld[y][x] = live
				}
			}
		}
		//sending the updated world to the distributor
		for y := 1; y < heightOfSmallerArrayWithExtraRows-1; y++ {
			for x := 0; x < width; x++ {
				channelout <- newWorld[y][x]
			}
		}
	}
}


//combines chunks of world created by workers into one world
func combine(workersOut [] chan byte, heightOfSmallerArray, width, threads int)[][]byte{

	world:=createMatrix(0, 0)

	for i:=0; i<threads; i++ {

		tempWorld:=createMatrix(heightOfSmallerArray, width)
		for y:=0; y<heightOfSmallerArray; y++ {
			for x:=0; x<width; x++ {
				val:=<-workersOut[i]
				tempWorld[y][x] = val
			}
		}
		world = append(world, tempWorld...)
	}
	return world
}



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

	world:= createMatrix(p.imageHeight, p.imageWidth)

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				world[y][x] = val
			}
		}
	}

	//creating slices of channels for workers
	workersIn := make([]chan byte, p.threads)
	workersOut := make([]chan byte, p.threads)

	//creating channels
	for i := range workersIn {
		workersIn[i] = make(chan byte)
		workersOut[i] = make(chan byte)
	}


	heightOfSmallerArray := p.imageHeight / p.threads

	//we only need to create the workers once
	for i:=0; i<p.threads; i++{
		go worker(workersIn[i], workersOut[i], heightOfSmallerArray, p.imageWidth)
	}

	//performing turns of the game
	for t:=0; t<p.turns; t++{

		//split array into parts for each worker
		split(p.imageHeight, heightOfSmallerArray, p.imageWidth, p.threads, workersIn, workersOut, world)
		//combine the results
		world = combine(workersOut, heightOfSmallerArray, p.imageWidth, p.threads)
	}





	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
				//fmt.Print(finalAlive)
			}
		}
	}

	//writing the pgm file
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x") + "_" + strconv.Itoa(p.turns) + "turns"
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
