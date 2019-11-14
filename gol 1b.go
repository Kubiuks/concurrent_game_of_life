package main

import (
	"fmt"
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



func split(height, width, thread int, channelsin, channelsout []chan byte, world [][]byte){
	heightOfSmallerArray := height/thread

	counter:=0
	for i:=0; i<thread; i++{
		go worker(channelsin[i], channelsout[i], heightOfSmallerArray, width )
	}
	/*for y:=0; y<height; y=y+heightOfSmallerArray{
		sendRow(width, (y-1+height)%height, world, channelsin[counter])
		counter++
	}

	counter=0
	*/
	for y:=0; y<height; y=y+heightOfSmallerArray {
		for h:=y-1; h<heightOfSmallerArray+1+y; h++ {
			for x:=0; x<width; x++ {
				var t int
				if h<0 {
					t=(h+height)%height
				} else if h>=height {
					t=(h-height)%height
				} else {
					t=h
				}
				channelsin[counter]<-world[t][x]
			}
		}
		counter++
	}


	/*for i:=0; i<thread; i++{
		for y:=0; y<height; y=y+heightOfSmallerArray{
			for h:=0; h<heightOfSmallerArray; h++{
				for x:=0; x<width; x++{
					channelsin[i]<-world[y+h][x]
				}
			}
		}
	}*/
	fmt.Print("all values sent")
/*
	for y:=0; y<height; y=y+heightOfSmallerArray{
		sendRow(width, (y+heightOfSmallerArray)%height, world, channelsin[counter])
		counter++
	}*/
}


func worker (channelin, channelout chan byte, heightOfSmallerArray, width int){
	heightOfSmallerArrayWithExtraRows:= heightOfSmallerArray+2

	world:=createMatrix(heightOfSmallerArrayWithExtraRows, width)
	newWorld:=createMatrix(heightOfSmallerArrayWithExtraRows, width)


	for y := 0; y < heightOfSmallerArrayWithExtraRows; y++ {
		for x := 0; x < width; x++ {
			val := <- channelin
			world[y][x] = val
			newWorld[y][x] = val
		}
	}
	for y := 1; y < heightOfSmallerArrayWithExtraRows-1; y++ {
		for x := 0; x < width; x++ {
			neighbors := getNumberOfNeighbors(world, y ,x, heightOfSmallerArrayWithExtraRows, width)
			if neighbors < 2 || neighbors > 3 {
				newWorld[y][x] = dead
			} else if neighbors == 3 {
				newWorld[y][x] = live
			}
		}
	}
	for y:=1; y<heightOfSmallerArrayWithExtraRows -1; y++{
		for x:=0; x<width; x++{
			channelout<-newWorld[y][x]
		}
	}
}

func combine(channelsout [] chan byte, heightOfSmallerArray, height, width, threads int)[][]byte{
	world:=createMatrix(height, width)

	for i:=0; i<threads; i++{
		tempWorld:=createMatrix(heightOfSmallerArray, width)
		for y:=0; y<heightOfSmallerArray; y++{
			for x:=0; x<width; x++{
				val:=<-channelsout[i]
				tempWorld[y][x] = val
				world = append(world, tempWorld...)
			}
		}
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
	newWorld:= createMatrix(p.imageHeight, p.imageWidth)


	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				//fmt.Println("Alive cell at", x, y)
				world[y][x] = val
				newWorld[y][x] = val
			}
		}
	}

	for t:=0; t<p.turns; t++{

		channelsin := make([]chan byte, p.threads)
		channelsout := make([]chan byte, p.threads)
		heightOfSmallerArray := p.imageHeight / p.threads

		for i := range channelsin {
			channelsin[i] = make(chan byte)
			channelsout[i] = make(chan byte)

		}
		finalworld := createMatrix(p.imageHeight, p.imageWidth)

		//split array into parts for each worker

		split(p.imageHeight, p.imageWidth, p.threads, channelsin, channelsout, newWorld)
		finalworld = combine(channelsout, heightOfSmallerArray, p.imageHeight, p.imageWidth, p.threads)

		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = finalworld[y][x]
				newWorld[y][x] = finalworld[y][x]
			}
		}
		//worker
		//combine results
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
