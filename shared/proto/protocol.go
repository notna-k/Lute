package worker

// Protocol is the wire protocol this build speaks. Core refuses agents below MinProtocol
// and tells them which image to pull; bump both when a change breaks older agents.
const (
	Protocol    int32 = 2
	MinProtocol int32 = 2
)
