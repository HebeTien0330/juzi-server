package common

type Code_ struct {
	SUCCESS int
	FAIL    int
}

var Code = Code_{
	SUCCESS: 200,
	FAIL:    500,
}
