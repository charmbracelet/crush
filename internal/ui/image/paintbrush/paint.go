package paintbrush

import (
	"sort"
	"sync"
)

// StartPainting begins the asynchronous painting process.
func (c *Canvas) StartPainting() {
	go func() {
		c.Paint()
		c.Progress = 1
	}()
}

// GetProgress returns the current progress of the painting process.
func (c *Canvas) GetProgress() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Progress
}

func (c *Canvas) renderWorker(wg *sync.WaitGroup, taskChan <-chan Task, resultChan chan<- TaskResult, imgCharWidth, imgCharHeight float64) {
	defer wg.Done()

	for task := range taskChan {
		result := c.processTask(task, imgCharWidth, imgCharHeight)
		resultChan <- result
	}
}

// Paint performs the synchronous painting process.
func (c *Canvas) Paint() {
	if len(c.Font.Glyphs) == 0 {
		fontBytes, err := EmbeddedFonts.ReadFile(FiraMonoRegular)
		if err != nil {
			return
		}
		err = c.SetFont(fontBytes)
		if err != nil {
			return
		}
	}

	c.Result = ""
	c.ResultRGBABytes = nil
	c.ResultC = ""
	c.ResultBash = ""

	width, height := c.calculateDimensions(c.Image.Bounds().Dx(), c.Image.Bounds().Dx())

	imgCharWidth := float64(int((float64(c.Image.Bounds().Dx())/float64(width))*float64(c.GlyphWidth))) / float64(c.GlyphWidth)
	imgCharHeight := float64(c.Image.Bounds().Dy()) / float64(height)

	c.ResultRGBAWidth = width * c.Font.GlyphWidth
	c.ResultRGBAHeight = height * c.Font.GlyphHeight
	c.ResultRGBABytes = make([]byte, c.ResultRGBAWidth*c.ResultRGBAHeight*4)

	tasks := make([]Task, 0, width*height)
	for charY := 0; charY < height; charY++ {
		for charX := 0; charX < width; charX++ {
			tasks = append(tasks, Task{CharX: charX, CharY: charY})
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		distFunc := func(t Task) float64 {
			dx := float64(t.CharX) - float64(width)/2
			dy := float64(t.CharY) - float64(height)/2
			return dx*dx/c.Font.Aspect + dy*dy*c.Font.Aspect
		}
		di, dj := distFunc(tasks[i]), distFunc(tasks[j])
		if di == dj {
			if tasks[i].CharX == tasks[j].CharX {
				return tasks[i].CharY < tasks[j].CharY
			}
			return tasks[i].CharX < tasks[j].CharX
		}
		return di > dj
	})

	taskResults := make([]TaskResult, len(tasks))
	var wg sync.WaitGroup
	taskChan := make(chan Task, len(tasks))
	resultChan := make(chan TaskResult, len(tasks))

	// Start worker goroutines
	for i := 0; i < c.Threads; i++ {
		wg.Add(1)
		go c.renderWorker(&wg, taskChan, resultChan, imgCharWidth, imgCharHeight)
	}

	// Feed tasks to workers
	go func() {
		for _, task := range tasks {
			taskChan <- task
		}
		close(taskChan)
	}()

	// Collect results only after every worker is done. Upstream collected
	// them in a goroutine that Paint never waited for, so processResults
	// could read cells before they were stored, leaving them blank or
	// panicking on a nil glyph. resultChan holds every result, so the
	// workers never block on it.
	wg.Wait()
	close(resultChan)
	i := 0
	for result := range resultChan {
		taskResults[result.CharY*width+result.CharX] = result
		i++
		c.mu.Lock()
		c.Progress = float32(i) / float32(len(tasks))
		c.mu.Unlock()
	}

	// Process results
	c.processResults(taskResults, width, height)
}

func (c *Canvas) calculateDimensions(imageX, imageY int) (width int, height int) {
	// Use local variables for calculations
	width = c.Width
	height = c.Height

	// Set default values if both width and height are 0
	if width == 0 && height == 0 {
		width, height = c.defaultDimensions()
	}

	aspectRatio := float64(imageX) / float64(imageY) * c.Font.Aspect

	if width == 0 {
		width = int(float64(height) * aspectRatio)
	} else if height == 0 {
		height = int(float64(width) / aspectRatio)
	} else {
		constrainedHeight := int(float64(width) / aspectRatio)
		constrainedWidth := int(float64(height) * aspectRatio)

		if constrainedHeight <= height {
			height = constrainedHeight
		} else {
			width = constrainedWidth
		}
	}

	return width, height
}

func (c *Canvas) defaultDimensions() (int, int) {
	return 40, 0
}
