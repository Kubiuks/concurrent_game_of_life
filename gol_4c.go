package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)


const dead = 0x00
const live  = 0xFF

type workerStruct struct {
	workerTurn 			 chan bool
	workerIn 			 chan byte
	workerOut			 chan byte
	workerTopLayerIn 	 <-chan byte
	workerBottomLayerIn  <-chan byte
	workerTopLayerOut 	 chan<- byte
	workerBottomLayerOut chan<- byte
}


func createWorkerStructSlice(threads int) []workerStruct{

	w := make([] workerStruct, threads)
	topSlice := make([]chan byte, threads)
	bottomSlice := make([]chan byte, threads)

	//creating channels
	for i:=0;i<threads;i++ {
		w[i].workerTurn 	  	  					  = make(chan bool)
		w[i].workerIn 	 		  					  = make(chan byte)
		w[i].workerOut 		      					  = make(chan byte)
		topSlice[i]				  					  = make(chan byte)
		bottomSlice[i]			  					  = make(chan byte)
		w[i].workerTopLayerIn	  					  = topSlice[i]
		w[(i-1+threads)%threads].workerBottomLayerOut = topSlice[i]
		w[i].workerBottomLayerIn  					  = bottomSlice[i]
		w[(i+1+threads)%threads].workerTopLayerOut 	  = bottomSlice[i]
	}

	return w
}

func getNumberOfNeighbors(world [][]byte, y,x,height,width int) int {
	neighbors := 0
	for i:=0; i<3;i++{
		for j:=0; j<3; j++{

			if world[(y-1+i+height)%height][(x-1+j+width)%width] == live{
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

// calculates the length each channel should hold and stores it in an array where the index indicates the channel.
// At first, every channel is given the maximum length that can be shared equally.
// The remaining is then added one by one to each channel. This way, work can be shared as equally as possible
func calculate(height, threads int)[]int{
	numbers := make([]int, threads)
	divide := int(math.Floor(float64(height / threads)))
	remaining := height - (threads*divide)
	for i:=range numbers{
		numbers[i] = divide
	}
	if remaining != 0{
		for i:=0; i<remaining; i++{
			numbers[i] ++
		}
	}
	return numbers
}


func createMatrix(height, width int) [][]byte{
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

//sending the appropriate data to workers
func splitSend(heightOfSmallerArray, width, height, y int, workerIn chan byte, world [][]byte) {

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
			workerIn <-world[t][x]
		}
	}
}

//splits the image into parts so workers can work on different parts in parallel
//heightSoFar stores the y-index each channel should start taking bytes from, and its updated each time.
func split(height, width int, w []workerStruct, world [][]byte, numbers []int){

	counter:=0
	heightSoFar:=0

	for i := range numbers {
		go splitSend(numbers[i], width, height, heightSoFar, w[counter].workerIn, world)
		counter ++
		heightSoFar = heightSoFar + numbers[i]
	}
	//splitting the process of sending the data to different workers
	/*for y:=0; y<height; y=y+heightOfSmallerArray {
		go splitSend(heightOfSmallerArray, width, height, y, counter, workersIn, world)
		counter++
	}*/
}


//Receive from workers concurrently
func combineReceive(number, width int, workerOut chan byte, tempWorlds chan [][]byte) {

	tempWorld:=createMatrix(number, width)
	for y:=0; y<number; y++ {
		for x:=0; x<width; x++ {
			val:=<-workerOut
			tempWorld[y][x] = val
		}
	}
	tempWorlds <- tempWorld
}


//combines chunks of world created by workers into one world using information from the number array
//it also returns the number of alive cells in the combined world
func combine(w []workerStruct, width, threads int, number []int, tempWorlds []chan [][]byte) [][]byte {

	world:=createMatrix(0, 0)

	for i:=0; i<threads; i++ {
		go combineReceive(number[i], width, w[i].workerOut, tempWorlds[i])
	}

	for i:=0; i<threads; i++ {
		world = append(world, <-tempWorlds[i]...)
	}


	return world
}


//performs the logic of the game on the given chunk
func worker (w workerStruct, me, heightOfSmallerArray, width int){

	heightOfSmallerArrayWithExtraRows := heightOfSmallerArray + 2

	world := createMatrix(heightOfSmallerArrayWithExtraRows, width)


	//we create a world for the next round in a new matrix
	//so the intermediate results dont affect current world
	//as this would change the output
	newWorld := createMatrix(heightOfSmallerArrayWithExtraRows, width)

	//initialise world and new world in current state only on first turn
	for y := 0; y < heightOfSmallerArrayWithExtraRows; y++ {
		for x := 0; x < width; x++ {
			val := <-w.workerIn
			world[y][x] = val
			newWorld[y][x] = val
		}
	}


	for {
		b := <-w.workerTurn
		// if we send true we perform a turn
		//if we send false we want to get the state of the world
		if b {
			//divide workers into even and odd so there is no circular wait
			//even first receive layers
			//odd first send
			if me%2 == 0 {
				for x := 0; x < width; x++ {
					world[heightOfSmallerArrayWithExtraRows-1][x] = <- w.workerBottomLayerIn
					world[0][x] = <- w.workerTopLayerIn
					w.workerTopLayerOut <- world[1][x]
					w.workerBottomLayerOut <- world[heightOfSmallerArrayWithExtraRows-2][x]
				}

			} else {
				for x := 0; x < width; x++ {
					w.workerTopLayerOut <- world[1][x]
					w.workerBottomLayerOut <- world[heightOfSmallerArrayWithExtraRows-2][x]
					world[heightOfSmallerArrayWithExtraRows-1][x] = <- w.workerBottomLayerIn
					world[0][x] = <- w.workerTopLayerIn
				}
			}

			//game logic
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

			// as we dont recreate the world after each round,
			// we need to update our world for next turn
			for y := 1; y < heightOfSmallerArrayWithExtraRows-1; y++ {
				for x := 0; x < width; x++ {
					world[y][x] = newWorld[y][x]
				}
			}

		} else {
			//sending the updated world to the distributor
			for y := 1; y < heightOfSmallerArrayWithExtraRows-1; y++ {
				for x := 0; x < width; x++ {
					w.workerOut <- world[y][x]
				}
			}
		}
	}
}


//send message to the workers to start performing turn
func doTurn(threads int, w []workerStruct, b bool) {
	for i:=0;i<threads;i++ {
		w[i].workerTurn <- b
	}
}


// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell, keyChan <-chan rune) {

	// Request the io goroutine to read in the image with the given filename.
	requestPGM(p,d)

	// creating the initial world from input image
	world := receiveWorld(p, d)


	// ticker sends a value through the channel every 2s
	ticker := time.NewTicker(2 * time.Second)

	//stores number of alive cells
	var numberOfAliveCells int

	//create number array
	number := calculate(p.imageHeight, p.threads)

	//slice of channels needed by each worker
	w := createWorkerStructSlice(p.threads)

	//temporary worlds created by getting data from each worker
	tempWorlds := make([] chan [][]byte, p.threads)

	//we only need to create the workers and tempWorld once (using information from number array)
	for i:=0; i<p.threads; i++{
		tempWorlds[i] = make(chan [][]byte)
		go worker(w[i], i, number[i], p.imageWidth)
	}

	split(p.imageHeight, p.imageWidth, w, world, number)

	t := 0
loop:
	for t<p.turns {
		select {
		//catching keyboard input
		case keyIn := <-keyChan :
			flag := keyLogic(keyIn, p, d, world, t, w, number, tempWorlds, keyChan)
			if flag {
				break loop
			}
		//prints number of cells when a value is sent through the channel
		case <-ticker.C:
			//number of alive cells in the processed/combined world
			numberOfAliveCells = countAlive(p, world, w, number, tempWorlds)
			fmt.Println("Number of alive cells =", numberOfAliveCells)
		//performing turn
		default:
			doTurn(p.threads, w, true)
			t++
		}
	}
	//number of alive cells in the processed/combined world


	//writing the pgm file after last calculated turn
	writePGM(p, d, world, t, w, number, tempWorlds)


	// cells still alive after last turn
	finalAlive := finalAlive(p, world, w, number, tempWorlds)

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}


func keyLogic(keyIn rune, p golParams, d distributorChans, world [][]byte, t int,
	w []workerStruct, number []int, tempWorlds []chan [][]byte, keyChan <-chan rune) bool {
	key := string(keyIn)
	if key == "s" {
		// generate a PGM file with the current state of the board
		fmt.Println("Outputting PGM at turn: " + strconv.Itoa(t))
		writePGM(p, d, world, t, w, number, tempWorlds)
	} else if key == "p" {
		//  pause the processing and print the current turn that is being processed.
		fmt.Println("Processing paused, current turn: " + strconv.Itoa(t))
		// after pausing you can either resume processing by pressing 'p' again,
		// press 's' to generate a PGM file at the stopped turn
		// or press 'q' to generate a PGM file at the turn and exit the program
		for {
			in2 := <-keyChan
			key2 := string(in2)
			if key2 == "p" {
				fmt.Println("Continuing")
				break
			} else if key2 == "s" {
				fmt.Println("Outputting PGM at turn: " + strconv.Itoa(t))
				writePGM(p, d, world, t, w, number, tempWorlds)
			} else if key2 == "q" {
				fmt.Println("Outputting PGM and terminating at turn: " + strconv.Itoa(t))
				//writePGM(p, d, world, t)
				return true
			}
		}
	} else if key == "q" {
		// generate a PGM file with the final state of the board and then terminate the program
		fmt.Println("Outputting PGM and terminating at turn: " + strconv.Itoa(t))
		//writePGM(p, d, world, t)
		return true
	}
	return false
}


func writePGM (p golParams, d distributorChans, world [][]byte, turn int,
	w []workerStruct, number []int, tempWorlds []chan [][]byte) {

	doTurn(p.threads, w, false)

	//combine the results and also update number of alive cells
	world = combine(w, p.imageWidth, p.threads, number, tempWorlds)

	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight), strconv.Itoa(p.threads)}, "x") + "_" + strconv.Itoa(turn) + "turns"
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outVal <- world[y][x]
		}
	}
}

func requestPGM(p golParams, d distributorChans)  {
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")
}

// The io goroutine sends the requested image byte by byte, in rows.
func receiveWorld(p golParams, d distributorChans) [][]byte {
	world:= createMatrix(p.imageHeight, p.imageWidth)
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				world[y][x] = val
			}
		}
	}
	return world
}

//returns a slice of slices that are alive at the end of the game
func finalAlive(p golParams, world [][]byte, w []workerStruct, number []int, tempWorlds []chan [][]byte) []cell {
	doTurn(p.threads, w, false)

	//combine the results and also update number of alive cells
	world = combine(w, p.imageWidth, p.threads, number, tempWorlds)

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
	return finalAlive
}

//counts number of alive cells in the given slice
func countAlive(p golParams, world [][]byte,
	w []workerStruct, number []int, tempWorlds []chan [][]byte) int {

	doTurn(p.threads, w, false)

	//combine the results and also update number of alive cells
	world = combine(w, p.imageWidth, p.threads, number, tempWorlds)

	var count = 0
	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				count++
			}
		}
	}
	return count
}