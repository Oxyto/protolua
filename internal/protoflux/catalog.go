package protoflux

import (
	"net/url"
	"sort"
	"strings"
)

type Node struct {
	Name        string   `json:"name"`
	Canonical   string   `json:"canonical"`
	Category    string   `json:"category,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Inputs      []string `json:"inputs,omitempty"`
	Outputs     []string `json:"outputs,omitempty"`
	Description string   `json:"description,omitempty"`
}

type ResolvedNode struct {
	Raw       string `json:"raw"`
	Path      string `json:"path"`
	Canonical string `json:"canonical"`
	Name      string `json:"name"`
	Category  string `json:"category,omitempty"`
	Known     bool   `json:"known"`
}

var catalog = []Node{
	node("Write", "Actions", []string{"Actions.Write"}, []string{"*", "Variable", "Value"}, []string{"Next"}),
	node("WriteLatch", "Actions", nil, []string{"*", "Set", "Reset"}, []string{"Next"}),
	node("WriteToGlobal", "Actions", nil, []string{"*", "Target", "Value"}, []string{"Next"}),
	node("WriteTextToFile", "Actions", nil, []string{"*", "Path", "Text"}, []string{"Next"}),
	node("Delay", "Actions", []string{"Actions.Delay"}, []string{"*", "Seconds"}, []string{"Next"}),
	node("Wait", "Actions", nil, []string{"*", "Seconds"}, []string{"Next"}),
	node("StartAsyncTask", "Actions", nil, []string{"*"}, []string{"Next"}),
	node("Call", "Actions", []string{"Actions.Call"}, []string{"*", "Delegate"}, []string{"Next"}),
	node("IndirectWrite", "Actions", nil, []string{"*", "Variable", "Value"}, []string{"Next"}),
	node("IndirectWriteLatch", "Actions", nil, []string{"*", "Set", "Reset"}, []string{"Next"}),
	node("Increment", "Actions", nil, []string{"*", "Variable"}, []string{"Next"}),
	node("Decrement", "Actions", nil, []string{"*", "Variable"}, []string{"Next"}),
	node("If", "Flow", []string{"Flow.If"}, []string{"*", "Condition"}, []string{"True", "False"}),
	node("For", "Flow", nil, []string{"*", "From", "To", "Step"}, []string{"LoopStart", "Iteration", "LoopEnd"}),
	node("While", "Flow", nil, []string{"*", "Condition"}, []string{"LoopStart", "LoopEnd"}),
	node("Sequence", "Flow", nil, []string{"*"}, []string{"Next0", "Next1", "Next2"}),
	node("FireOnTrue", "Flow", nil, []string{"Condition"}, []string{"OnTrue"}),
	node("FireOnFalse", "Flow", nil, []string{"Condition"}, []string{"OnFalse"}),
	node("FireWhileTrue", "Flow", nil, []string{"Condition"}, []string{"WhileTrue"}),
	node("FireOnLocalTrue", "Flow", nil, []string{"Condition"}, []string{"OnTrue"}),
	node("FireOnLocalFalse", "Flow", nil, []string{"Condition"}, []string{"OnFalse"}),
	node("OncePerFrame", "Flow", nil, []string{"*"}, []string{"Next"}),
	node("ImpulseMultiplexer", "Flow", nil, []string{"Index"}, []string{"*"}),
	node("ImpulseDemultiplexer", "Flow", nil, []string{"*"}, []string{"0", "1", "2"}),
	node("BooleanCounter", "Flow", nil, []string{"*", "Reset"}, []string{"Count"}),
	node("ButtonEvents", "Interaction", nil, []string{"Button"}, []string{"Pressed", "Pressing", "Released"}),
	node("GrabbableEvents", "Interaction", nil, []string{"Grabbable"}, []string{"Grabbed", "Released"}),
	node("TouchableEvents", "Interaction", nil, []string{"Touchable"}, []string{"Touched", "Touching", "Released"}),
	node("ContextMenuItem", "Interaction", nil, []string{"Name", "Color"}, []string{"Selected"}),
	node("LocalToGlobal", "Transform", nil, []string{"Local", "Space"}, []string{"Global"}),
	node("GlobalPointToLocal", "Transform", nil, []string{"Global", "Space"}, []string{"Local"}),
	node("GlobalVectorToLocal", "Transform", nil, []string{"Global", "Space"}, []string{"Local"}),
	node("GlobalDirectionToLocal", "Transform", nil, []string{"Global", "Space"}, []string{"Local"}),
	node("GlobalRotationToLocal", "Transform", nil, []string{"Global", "Space"}, []string{"Local"}),
	node("GlobalScaleToLocal", "Transform", nil, []string{"Global", "Space"}, []string{"Local"}),
	node("GlobalTransform", "Transform", nil, []string{"Slot"}, []string{"Position", "Rotation", "Scale"}),
	node("BoundingBoxProperties", "Transform", nil, []string{"Box"}, []string{"Center", "Size"}),
	node("RootSlot", "Slots", []string{"Slots.RootSlot"}, nil, []string{"Slot"}),
	node("ThisSlot", "Slots", nil, nil, []string{"Slot"}),
	node("ParentSlot", "Slots", nil, []string{"Slot"}, []string{"Parent"}),
	node("Children", "Slots", nil, []string{"Slot"}, []string{"Children"}),
	node("ChildSlot", "Slots", nil, []string{"Slot", "Name"}, []string{"Child"}),
	node("FindChildByName", "Slots", []string{"FindSlot"}, []string{"Source", "Name"}, []string{"Slot"}),
	node("GetSlotName", "Slots", nil, []string{"Slot"}, []string{"Name"}),
	node("GetSlotActiveSelf", "Slots", nil, []string{"Slot"}, []string{"Active"}),
	node("GetSlotPersistent", "Slots", nil, []string{"Slot"}, []string{"Persistent"}),
	node("GetSlotPersistentSelf", "Slots", nil, []string{"Slot"}, []string{"PersistentSelf"}),
	node("GetSlotOrderOffset", "Slots", nil, []string{"Slot"}, []string{"OrderOffset"}),
	node("GetTag", "Slots", nil, []string{"Slot"}, []string{"Tag"}),
	node("IndexOfChild", "Slots", nil, []string{"Parent", "Child"}, []string{"Index"}),
	node("SetSlotActive", "Slots", []string{"SetActive"}, []string{"*", "Slot", "Active"}, []string{"Next"}),
	node("DestroySlot", "Slots", []string{"Destroy"}, []string{"*", "Slot"}, []string{"Next"}),
	node("GetUserFromComponent", "Components", nil, []string{"Component"}, []string{"User"}),
	node("GetSlot", "Components", nil, []string{"Component"}, []string{"Slot"}),
	node("ComponentEnabled", "Components", []string{"ComponentEnabledSource"}, []string{"Component"}, []string{"Enabled"}),
	node("SetComponentEnabled", "Components", nil, []string{"*", "Component", "Enabled"}, []string{"Next"}),
	node("FieldAsVariable", "Fields", []string{"Fields.FieldAsVariable"}, []string{"Field"}, []string{"Variable"}),
	node("ReferenceAsVariable", "Fields", nil, []string{"Reference"}, []string{"Variable"}),
	node("ReferenceToOutput", "Fields", nil, []string{"Reference"}, []string{"Value"}),
	node("GlobalToOutput", "Fields", nil, []string{"Global"}, []string{"Value"}),
	node("BaseValue", "Fields", nil, []string{"Variable"}, []string{"Value"}),
	node("ReadDynamicVariable", "Variables.Dynamic", []string{"Variables.Dynamic.ReadDynamicVariable"}, []string{"Source", "Path", "Type"}, []string{"Value"}),
	node("WriteDynamicVariable", "Variables.Dynamic", []string{"Variables.Dynamic.WriteDynamicVariable"}, []string{"*", "Target", "Path", "Value"}, []string{"Next"}),
	node("CreateDynamicVariable", "Variables.Dynamic", []string{"Variables.Dynamic.CreateDynamicVariable"}, []string{"*", "Target", "Path", "InitialValue"}, []string{"Next"}),
	node("WriteOrCreateDynamicVariable", "Variables.Dynamic", []string{"Variables.Dynamic.WriteOrCreateDynamicVariable"}, []string{"*", "Target", "Path", "Value"}, []string{"Next"}),
	node("DynamicVariableInput", "Variables.Dynamic", []string{"Variables.Dynamic.DynamicVariableInput"}, []string{"Path", "Type"}, []string{"Value"}),
	node("DynamicVariableInputWithEvents", "Variables.Dynamic", []string{"Variables.Dynamic.DynamicVariableInputWithEvents"}, []string{"Path", "Type"}, []string{"Value", "OnChanged"}),
	node("DynamicVariableSpace", "Variables.Dynamic", []string{"Variables.Dynamic.DynamicVariableSpace"}, []string{"*", "Slot", "Name"}, []string{"Next"}),
	node("DynamicVariableDriver", "Variables.Dynamic", nil, []string{"*", "Target", "Path", "Field"}, []string{"Next"}),
	node("DeleteDynamicVariable", "Variables.Dynamic", nil, []string{"*", "Target", "Path", "Type"}, []string{"Next"}),
	node("ClearDynamicVariables", "Variables.Dynamic", nil, []string{"*", "Target"}, []string{"Next"}),
	node("ClearDynamicVariablesOfType", "Variables.Dynamic", nil, []string{"*", "Target", "Type"}, []string{"Next"}),
	node("ReadCloudVariable", "Variables.Cloud", nil, []string{"Path"}, []string{"Value"}),
	node("WriteCloudVariable", "Variables.Cloud", nil, []string{"*", "Path", "Value"}, []string{"Next"}),
	node("DebugLog", "Debug", []string{"Debug.DebugLog"}, []string{"Value"}, nil),
	node("Stopwatch", "Debug", nil, []string{"*"}, []string{"Elapsed"}),
	node("ImpulseDisplay", "Debug", nil, []string{"*"}, []string{"Next"}),
	node("Display", "Debug", nil, []string{"Value"}, nil),
	node("FormatString", "Strings", []string{"String.FormatString"}, []string{"Format", "Args"}, []string{"String"}),
	node("FormatLocaleString", "Strings", nil, []string{"Format", "Args"}, []string{"String"}),
	node("FormatQuantity", "Strings", nil, []string{"Quantity"}, []string{"String"}),
	node("StringLength", "Strings", nil, []string{"String"}, []string{"Length"}),
	node("StringInsert", "Strings", nil, []string{"String", "Index", "Value"}, []string{"String"}),
	node("StringRemove", "Strings", nil, []string{"String", "Index", "Count"}, []string{"String"}),
	node("Substring", "Strings", nil, []string{"String", "Start", "Length"}, []string{"String"}),
	node("ReplaceSubstring", "Strings", nil, []string{"String", "Old", "New"}, []string{"String"}),
	node("ReplaceFirstSubstring", "Strings", nil, []string{"String", "Old", "New"}, []string{"String"}),
	node("StartsWith", "Strings", nil, []string{"String", "Prefix"}, []string{"Value"}),
	node("EndsWith", "Strings", nil, []string{"String", "Suffix"}, []string{"Value"}),
	node("ContainsString", "Strings", nil, []string{"String", "Value"}, []string{"Value"}),
	node("IndexOfString", "Strings", nil, []string{"String", "Value"}, []string{"Index"}),
	node("Capitalize", "Strings", nil, []string{"String"}, []string{"String"}),
	node("ReverseString", "Strings", nil, []string{"String"}, []string{"String"}),
	node("StripRTFTags", "Strings", nil, []string{"String"}, []string{"String"}),
	node("AND", "Operators.Boolean", []string{"Operators.AND"}, []string{"A", "B"}, []string{"*"}),
	node("OR", "Operators.Boolean", []string{"Operators.OR"}, []string{"A", "B"}, []string{"*"}),
	node("XOR", "Operators.Boolean", []string{"Operators.XOR"}, []string{"A", "B"}, []string{"*"}),
	node("XNOR", "Operators.Boolean", []string{"Operators.XNOR"}, []string{"A", "B"}, []string{"*"}),
	node("NOT", "Operators.Boolean", []string{"Operators.NOT"}, []string{"Value"}, []string{"*"}),
	node("Add", "Operators", []string{"Operators.Add", "ValueAddMulti", "ValueAdd"}, []string{"A", "B"}, []string{"*"}),
	node("Sub", "Operators", []string{"Operators.Sub", "ValueSubMulti"}, []string{"A", "B"}, []string{"*"}),
	node("Mul", "Operators", []string{"Operators.Mul", "ValueMulMulti"}, []string{"A", "B"}, []string{"*"}),
	node("Div", "Operators", []string{"Operators.Div", "ValueDivMulti"}, []string{"A", "B"}, []string{"*"}),
	node("Mod", "Operators", []string{"Operators.Mod", "ValueMod"}, []string{"A", "B"}, []string{"*"}),
	node("Pow", "Operators", []string{"Operators.Pow"}, []string{"A", "B"}, []string{"*"}),
	node("Avg", "Operators", nil, []string{"A", "B"}, []string{"*"}),
	node("AvgMulti", "Operators", nil, []string{"Values"}, []string{"*"}),
	node("ValueAbs", "Operators", nil, []string{"Value"}, []string{"*"}),
	node("ValueClamp", "Operators", nil, []string{"Value", "Min", "Max"}, []string{"*"}),
	node("ValueLerp", "Operators", nil, []string{"A", "B", "T"}, []string{"*"}),
	node("ValueLerpUnclamped", "Operators", nil, []string{"A", "B", "T"}, []string{"*"}),
	node("ValueMin", "Operators", nil, []string{"A", "B"}, []string{"*"}),
	node("ValueMax", "Operators", nil, []string{"A", "B"}, []string{"*"}),
	node("ValueMinMulti", "Operators", nil, []string{"Values"}, []string{"*"}),
	node("ValueMaxMulti", "Operators", nil, []string{"Values"}, []string{"*"}),
	node("ValueNegate", "Operators", nil, []string{"Value"}, []string{"*"}),
	node("ValueReciprocal", "Operators", nil, []string{"Value"}, []string{"*"}),
	node("ValueRepeat", "Operators", nil, []string{"Value", "Length"}, []string{"*"}),
	node("ValueFilterInvalid", "Operators", nil, []string{"Value", "Fallback"}, []string{"*"}),
	node("Pack", "Operators", []string{"PackValue"}, []string{"X", "Y", "Z", "W"}, []string{"*"}),
	node("Unpack", "Operators", []string{"UnpackValue"}, []string{"Value"}, []string{"X", "Y", "Z", "W"}),
	node("Equal", "Operators", []string{"Operators.Equal"}, []string{"A", "B"}, []string{"*"}),
	node("NotEqual", "Operators", []string{"Operators.NotEqual"}, []string{"A", "B"}, []string{"*"}),
	node("GreaterThan", "Operators", []string{"Operators.GreaterThan"}, []string{"A", "B"}, []string{"*"}),
	node("GreaterOrEqual", "Operators", []string{"Operators.GreaterOrEqual"}, []string{"A", "B"}, []string{"*"}),
	node("LessThan", "Operators", []string{"Operators.LessThan"}, []string{"A", "B"}, []string{"*"}),
	node("LessOrEqual", "Operators", []string{"Operators.LessOrEqual"}, []string{"A", "B"}, []string{"*"}),
	node("Floor", "Math", nil, []string{"Value"}, []string{"*"}),
	node("FloorToInt", "Math", []string{"FloortoInt"}, []string{"Value"}, []string{"*"}),
	node("Ceil", "Math", nil, []string{"Value"}, []string{"*"}),
	node("CeilToInt", "Math", nil, []string{"Value"}, []string{"*"}),
	node("Round", "Math", nil, []string{"Value"}, []string{"*"}),
	node("RoundToInt", "Math", nil, []string{"Value"}, []string{"*"}),
	node("Sqrt", "Math", nil, []string{"Value"}, []string{"*"}),
	node("Factorial", "Math", nil, []string{"Value"}, []string{"*"}),
	node("GreatestCommonDivisor", "Math", nil, []string{"A", "B"}, []string{"*"}),
	node("Remap", "Math", nil, []string{"Value", "FromMin", "FromMax", "ToMin", "ToMax"}, []string{"*"}),
	node("Remap1101", "Math", nil, []string{"Value"}, []string{"*"}),
	node("Repeat01", "Math", nil, []string{"Value"}, []string{"*"}),
	node("ZeroOne", "Math", nil, []string{"Value"}, []string{"*"}),
	node("Sin", "Math.Trigonometry", nil, []string{"Radians"}, []string{"*"}),
	node("Cos", "Math.Trigonometry", nil, []string{"Radians"}, []string{"*"}),
	node("Tan", "Math.Trigonometry", nil, []string{"Radians"}, []string{"*"}),
	node("Asin", "Math.Trigonometry", nil, []string{"Value"}, []string{"*"}),
	node("Acos", "Math.Trigonometry", nil, []string{"Value"}, []string{"*"}),
	node("Atan", "Math.Trigonometry", nil, []string{"Value"}, []string{"*"}),
	node("Atan2", "Math.Trigonometry", nil, []string{"Y", "X"}, []string{"*"}),
	node("Rad2Deg", "Math.Trigonometry", nil, []string{"Radians"}, []string{"Degrees"}),
	node("Deg2Rad", "Math.Trigonometry", nil, []string{"Degrees"}, []string{"Radians"}),
	node("BezierCurve", "Math.Geometry", nil, []string{"A", "B", "C", "D", "T"}, []string{"Point"}),
	node("AxisAngle", "Math.Geometry", nil, []string{"Axis", "Angle"}, []string{"Rotation"}),
	node("Project", "Operators.Vectors", nil, []string{"A", "B"}, []string{"*"}),
	node("Reflect", "Operators.Vectors", nil, []string{"Direction", "Normal"}, []string{"*"}),
	node("Angle", "Operators.Vectors", nil, []string{"A", "B"}, []string{"*"}),
	node("SqrMagnitude", "Operators.Vectors", nil, []string{"Value"}, []string{"*"}),
	node("PointOnCircle", "Operators.Vectors", nil, []string{"Angle", "Radius"}, []string{"Point"}),
	node("PointOnUVSphere", "Operators.Vectors", nil, []string{"U", "V", "Radius"}, []string{"Point"}),
	node("OrientationOnUVSphere", "Operators.Vectors", nil, []string{"U", "V"}, []string{"Rotation"}),
	node("FromEuler", "Rotation", nil, []string{"Euler"}, []string{"Rotation"}),
	node("FromToRotation", "Rotation", nil, []string{"From", "To"}, []string{"Rotation"}),
	node("RotationAtTargetPoint", "Rotation", nil, []string{"Source", "Target"}, []string{"Rotation"}),
	node("RandomBool", "Random", nil, []string{"Seed"}, []string{"Value"}),
	node("RandomInt", "Random", nil, []string{"Min", "Max", "Seed"}, []string{"Value"}),
	node("RandomFloat", "Random", nil, []string{"Min", "Max", "Seed"}, []string{"Value"}),
	node("RandomFloat2", "Random", nil, []string{"Min", "Max", "Seed"}, []string{"Value"}),
	node("RandomFloat3", "Random", nil, []string{"Min", "Max", "Seed"}, []string{"Value"}),
	node("RandomFloat4", "Random", nil, []string{"Min", "Max", "Seed"}, []string{"Value"}),
	node("RandomEnum", "Random", nil, []string{"Seed"}, []string{"Value"}),
	node("RandomGUID", "Random", nil, []string{"Seed"}, []string{"Value"}),
	node("RandomHueColor", "Random", nil, []string{"Seed"}, []string{"Color"}),
	node("RandomHueColorX", "Random", nil, []string{"Seed"}, []string{"Color"}),
	node("RandomGrayscaleColor", "Random", nil, []string{"Seed"}, []string{"Color"}),
	node("RandomGrayscaleColorX", "Random", nil, []string{"Seed"}, []string{"Color"}),
	node("Box", "Physics", nil, []string{"Center", "Size"}, []string{"Box"}),
	node("Raycaster", "Physics", nil, []string{"Origin", "Direction"}, []string{"Hit"}),
	node("RaycastOne", "Physics", nil, []string{"Origin", "Direction", "MaxDistance"}, []string{"Hit"}),
	node("RayPlaneIntersection", "Physics", nil, []string{"Ray", "Plane"}, []string{"Point"}),
	node("RayRectangleIntersection", "Physics", nil, []string{"Ray", "Rectangle"}, []string{"Point"}),
	node("RaySphereIntersection", "Physics", nil, []string{"Ray", "Sphere"}, []string{"Point"}),
	node("HitUVCoordinate", "Physics", nil, []string{"Hit"}, []string{"UV"}),
	node("CharacterControllerUser", "Physics", nil, []string{"Controller"}, []string{"User"}),
	node("AttachAudioClip", "Assets", nil, []string{"*", "Slot", "Clip"}, []string{"Next"}),
	node("AttachMesh", "Assets", nil, []string{"*", "Slot", "Mesh"}, []string{"Next"}),
	node("AttachSprite", "Assets", nil, []string{"*", "Slot", "Sprite"}, []string{"Next"}),
	node("AttachTexture2D", "Assets", nil, []string{"*", "Slot", "Texture"}, []string{"Next"}),
	node("PlayOneShot", "Audio", nil, []string{"*", "Clip", "Volume"}, []string{"Next"}),
	node("Play", "Media", nil, []string{"*", "Playable"}, []string{"Next"}),
	node("Pause", "Media", nil, []string{"*", "Playable"}, []string{"Next"}),
	node("Stop", "Media", nil, []string{"*", "Playable"}, []string{"Next"}),
	node("PlaybackPosition", "Media", nil, []string{"Playable"}, []string{"Position"}),
	node("PlaybackState", "Media", nil, []string{"Playable"}, []string{"State"}),
	node("HSLToColor", "Colors", nil, []string{"H", "S", "L", "A"}, []string{"Color"}),
	node("HSLToColorX", "Colors", nil, []string{"H", "S", "L", "A"}, []string{"Color"}),
	node("HSVToColor", "Colors", nil, []string{"H", "S", "V", "A"}, []string{"Color"}),
	node("HSVToColorX", "Colors", nil, []string{"H", "S", "V", "A"}, []string{"Color"}),
	node("BlackBodyColor", "Colors", nil, []string{"Temperature"}, []string{"Color"}),
	node("BlackBodyColorX", "Colors", nil, []string{"Temperature"}, []string{"Color"}),
	node("WavelengthColor", "Colors", nil, []string{"Wavelength"}, []string{"Color"}),
	node("WavelengthColorX", "Colors", nil, []string{"Wavelength"}, []string{"Color"}),
	node("Backspace", "Strings.Characters", nil, nil, []string{"Character"}),
	node("CarriageReturn", "Strings.Characters", nil, nil, []string{"Character"}),
	node("Bell", "Strings.Characters", nil, nil, []string{"Character"}),
	node("LocalUser", "Users", nil, nil, []string{"User"}),
	node("HostUser", "Users", nil, nil, []string{"User"}),
	node("UserRoot", "Users", nil, []string{"User"}, []string{"Slot"}),
	node("UserID", "Users", nil, []string{"User"}, []string{"ID"}),
	node("UserName", "Users", nil, []string{"User"}, []string{"Name"}),
	node("UserUserType", "Users", nil, []string{"User"}, []string{"Type"}),
	node("UserPing", "Users", nil, []string{"User"}, []string{"Ping"}),
	node("UserFPS", "Users", nil, []string{"User"}, []string{"FPS"}),
	node("HeadSlot", "Avatars.Body", nil, []string{"User"}, []string{"Slot"}),
	node("HeadPosition", "Avatars.Body", nil, []string{"User"}, []string{"Position"}),
	node("HeadRotation", "Avatars.Body", nil, []string{"User"}, []string{"Rotation"}),
	node("HeadFacingDirection", "Avatars.Body", nil, []string{"User"}, []string{"Direction"}),
	node("LeftHandSlot", "Avatars.Body", nil, []string{"User"}, []string{"Slot"}),
	node("RightHandSlot", "Avatars.Body", nil, []string{"User"}, []string{"Slot"}),
	node("HipsPosition", "Avatars.Body", nil, []string{"User"}, []string{"Position"}),
	node("HipsRotation", "Avatars.Body", nil, []string{"User"}, []string{"Rotation"}),
	node("WorldName", "World", nil, nil, []string{"Name"}),
	node("WorldDescription", "World", nil, nil, []string{"Description"}),
	node("WorldPath", "World", nil, nil, []string{"Path"}),
	node("WorldRecordURL", "World", nil, nil, []string{"URL"}),
	node("WorldSessionID", "World", nil, nil, []string{"ID"}),
	node("WorldSessionURL", "World", nil, nil, []string{"URL"}),
	node("WorldSessionWebURL", "World", nil, nil, []string{"URL"}),
	node("WorldWebURL", "World", nil, nil, []string{"URL"}),
	node("WorldTimeFloat", "Time", nil, nil, []string{"Time"}),
	node("WorldTimeDouble", "Time", nil, nil, []string{"Time"}),
	node("DeltaTime", "Time", nil, nil, []string{"Delta"}),
	node("DateTimeNow", "Time", nil, nil, []string{"DateTime"}),
	node("UtcNow", "Time", nil, nil, []string{"DateTime"}),
	node("TimeSpanFromSeconds", "Time", nil, []string{"Seconds"}, []string{"TimeSpan"}),
	node("BeginUndoBatch", "Undo", nil, []string{"*", "User", "Name"}, []string{"Next"}),
	node("EndUndoBatch", "Undo", nil, []string{"*", "User"}, []string{"Next"}),
	node("UndoBatch", "Undo", nil, []string{"*", "User", "Name"}, []string{"Next"}),
	node("WebsocketConnect", "Network.Websocket", nil, []string{"*", "URL"}, []string{"Connection"}),
	node("WebsocketConnectionEvents", "Network.Websocket", nil, []string{"Connection"}, []string{"Connected", "Disconnected", "Error"}),
	node("WebsocketTextMessageReceiver", "Network.Websocket", nil, []string{"Connection"}, []string{"Message"}),
	node("WebsocketTextMessageSender", "Network.Websocket", nil, []string{"*", "Connection", "Message"}, []string{"Next"}),
	node("HTTPGetString", "Network.HTTP", nil, []string{"*", "URL"}, []string{"Response", "Error"}),
	node("HTTPPostString", "Network.HTTP", nil, []string{"*", "URL", "Body"}, []string{"Response", "Error"}),
	node("WorldAccessLevel", "Security", nil, nil, []string{"AccessLevel"}),
	node("CanBeGrabbed", "Tools", nil, []string{"Grabbable"}, []string{"Value"}),
	node("EquippedTool", "Tools", nil, []string{"User"}, []string{"Tool"}),
	node("PackProtoFluxFromNode", "Nodes", nil, []string{"*", "Target", "Node"}, []string{"Next"}),
	node("PackProtoFluxInPlace", "Nodes", nil, []string{"*", "Node"}, []string{"Next"}),
	node("PackProtoFluxNodes", "Nodes", nil, []string{"*", "Target", "Nodes"}, []string{"Next"}),
	node("UnpackProtoFlux", "Nodes", nil, []string{"*", "Slot"}, []string{"Next"}),
	node("FunctionProxy", "", nil, []string{"Target", "Args"}, []string{"Result"}),
	node("MethodProxy", "", nil, []string{"Target", "Args"}, []string{"Result"}),
}

var index = buildIndex(catalog)

func All() []Node {
	out := append([]Node(nil), catalog...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Name < out[j].Name
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func Search(query string, limit int) []Node {
	queryKey := lookupKey(query)
	nodes := All()
	if queryKey == "" {
		return limitNodes(nodes, limit)
	}
	type scored struct {
		node  Node
		score int
	}
	matches := []scored{}
	seen := map[string]bool{}
	for _, n := range nodes {
		score := matchScore(n, queryKey)
		if score == 0 {
			continue
		}
		if seen[n.Canonical] {
			continue
		}
		seen[n.Canonical] = true
		matches = append(matches, scored{node: n, score: score})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].node.Name < matches[j].node.Name
		}
		return matches[i].score > matches[j].score
	})
	out := make([]Node, 0, len(matches))
	for _, match := range matches {
		out = append(out, match.node)
	}
	return limitNodes(out, limit)
}

func Lookup(raw string) (Node, bool) {
	resolved := Resolve(raw)
	if !resolved.Known {
		return Node{}, false
	}
	node, ok := index[lookupKey(resolved.Canonical)]
	return node, ok
}

func Resolve(raw string) ResolvedNode {
	cleaned := CleanPath(raw)
	if cleaned == "" {
		return ResolvedNode{Raw: raw}
	}
	if n, ok := index[lookupKey(cleaned)]; ok {
		return resolvedKnown(raw, cleaned, n)
	}
	name, category := splitPath(cleaned)
	canonicalName := name
	if canonicalName == "" {
		canonicalName = cleaned
	}
	return ResolvedNode{
		Raw:       raw,
		Path:      cleaned,
		Canonical: "ProtoFlux:" + canonicalName,
		Name:      canonicalName,
		Category:  category,
		Known:     false,
	}
}

func Normalize(raw string) string {
	return Resolve(raw).Path
}

func CleanPath(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(cleaned); err == nil {
		cleaned = decoded
	}
	if parsed, err := url.Parse(cleaned); err == nil && parsed.Host != "" {
		if title := parsed.Query().Get("title"); title != "" {
			cleaned = title
		} else {
			cleaned = strings.Trim(parsed.Path, "/")
		}
	}
	cleaned = strings.TrimPrefix(cleaned, "index.php/")
	cleaned = strings.TrimPrefix(cleaned, "wiki/")
	cleaned = strings.TrimPrefix(cleaned, "Category:")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.TrimPrefix(cleaned, ":")
	if strings.HasPrefix(cleaned, "ProtoFlux/") {
		cleaned = "ProtoFlux:" + strings.TrimPrefix(cleaned, "ProtoFlux/")
	}
	return cleaned
}

func node(name, category string, aliases, inputs, outputs []string) Node {
	canonical := "ProtoFlux:" + name
	allAliases := append([]string{}, aliases...)
	if category != "" {
		allAliases = append(allAliases, category+"."+name)
	}
	return Node{
		Name:      name,
		Canonical: canonical,
		Category:  category,
		Aliases:   dedupeStrings(allAliases),
		Inputs:    dedupeStrings(inputs),
		Outputs:   dedupeStrings(outputs),
	}
}

func buildIndex(nodes []Node) map[string]Node {
	out := map[string]Node{}
	for _, n := range nodes {
		keys := []string{n.Name, n.Canonical, "ProtoFlux:" + n.Name}
		if n.Category != "" {
			keys = append(keys, n.Category+"."+n.Name)
		}
		keys = append(keys, n.Aliases...)
		for _, key := range keys {
			normalized := lookupKey(key)
			if normalized == "" {
				continue
			}
			if _, exists := out[normalized]; !exists {
				out[normalized] = n
			}
		}
	}
	return out
}

func resolvedKnown(raw, cleaned string, n Node) ResolvedNode {
	return ResolvedNode{
		Raw:       raw,
		Path:      preferredPath(cleaned, n),
		Canonical: n.Canonical,
		Name:      n.Name,
		Category:  n.Category,
		Known:     true,
	}
}

func preferredPath(cleaned string, n Node) string {
	if cleaned != "" && !strings.HasPrefix(cleaned, "ProtoFlux:") && strings.Contains(cleaned, ".") {
		return cleaned
	}
	for _, alias := range n.Aliases {
		if strings.Contains(alias, ".") {
			return alias
		}
	}
	if n.Category != "" {
		return n.Category + "." + n.Name
	}
	return n.Canonical
}

func splitPath(path string) (string, string) {
	if strings.HasPrefix(path, "ProtoFlux:") {
		return strings.TrimPrefix(path, "ProtoFlux:"), ""
	}
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:], path[:i]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:], strings.ReplaceAll(path[:i], "/", ".")
	}
	return path, ""
}

func lookupKey(value string) string {
	value = CleanPath(value)
	value = strings.ToLower(value)
	value = strings.TrimPrefix(value, "protoflux:")
	value = strings.ReplaceAll(value, "/", ".")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func matchScore(n Node, queryKey string) int {
	candidates := append([]string{n.Name, n.Canonical, n.Category + "." + n.Name}, n.Aliases...)
	best := 0
	for _, candidate := range candidates {
		key := lookupKey(candidate)
		score := 0
		switch {
		case key == queryKey:
			score = 100
		case strings.HasPrefix(key, queryKey):
			score = 80
		case strings.Contains(key, queryKey):
			score = 50
		}
		if score > best {
			best = score
		}
	}
	return best
}

func limitNodes(nodes []Node, limit int) []Node {
	if limit <= 0 || limit >= len(nodes) {
		return nodes
	}
	return nodes[:limit]
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
