package chat

import "time"

func weatherTool() string {
	// simulate delay
	time.Sleep(2 * time.Second)
	return "Temprerature: 22°C, Condition: Sunny"
}

func timeTool() string {
	return "Current Time: 14:30 PM"
}
