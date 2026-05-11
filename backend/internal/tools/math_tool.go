package tools

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// NewMathTool creates the calculate_math tool
func NewMathTool() *Tool {
	return &Tool{
		Name:        "calculate_math",
		DisplayName: "Calculate Math",
		Description: "Evaluate advanced mathematical expressions safely. Supports arithmetic operations (+, -, *, /, ^, %), mathematical functions (sin, cos, tan, sqrt, log, ln, abs, floor, ceil, round), constants (pi, e), and parentheses for grouping. Examples: 'sqrt(16) + 2^3', 'sin(pi/2) * 10', 'log(100) + abs(-5)'",
		Icon:        "Calculator",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "Mathematical expression to evaluate (e.g., '2 + 2', 'sqrt(16) * pi', 'sin(pi/4) + cos(pi/4)')",
				},
			},
			"required": []string{"expression"},
		},
		Execute:  executeCalculateMath,
		Source:   ToolSourceBuiltin,
		Category: "computation",
		Keywords: []string{"math", "calculate", "compute", "arithmetic", "formula", "expression", "equation", "algebra", "trigonometry", "sqrt", "sin", "cos"},
	}
}

func executeCalculateMath(args map[string]interface{}) (string, error) {
	expression, ok := args["expression"].(string)
	if !ok || expression == "" {
		return "", fmt.Errorf("expression parameter is required and must be a string")
	}

	// Validate expression for safety
	if err := validateMathExpression(expression); err != nil {
		return "", fmt.Errorf("invalid expression: %w", err)
	}

	// Evaluate the expression
	result, err := evaluateMathExpression(expression)
	if err != nil {
		return "", fmt.Errorf("evaluation error: %w", err)
	}

	return fmt.Sprintf("Result: %v\n\nExpression: %s", result, expression), nil
}

// validateMathExpression checks if the expression is safe to evaluate
func validateMathExpression(expr string) error {
	// Remove whitespace for easier validation
	expr = strings.ReplaceAll(expr, " ", "")

	// Check for dangerous patterns (code execution attempts)
	dangerousPatterns := []string{
		"__", "exec", "eval", "import", "system", "os.",
		"file", "open", "read", "write", "delete", "rm",
		";", "&&", "||", "`", "$", "\\",
	}

	lowerExpr := strings.ToLower(expr)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerExpr, pattern) {
			return fmt.Errorf("expression contains forbidden pattern: %s", pattern)
		}
	}

	// Validate allowed characters only
	allowedPattern := regexp.MustCompile(`^[0-9+\-*/^()., a-zA-Z]+$`)
	if !allowedPattern.MatchString(expr) {
		return fmt.Errorf("expression contains invalid characters")
	}

	// Check for balanced parentheses
	if !hasBalancedParentheses(expr) {
		return fmt.Errorf("unbalanced parentheses")
	}

	return nil
}

// hasBalancedParentheses checks if parentheses are balanced
func hasBalancedParentheses(expr string) bool {
	count := 0
	for _, char := range expr {
		if char == '(' {
			count++
		} else if char == ')' {
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

// evaluateMathExpression safely evaluates a mathematical expression
func evaluateMathExpression(expr string) (float64, error) {
	// Preprocess: replace constants
	expr = strings.ReplaceAll(expr, " ", "")
	expr = replaceConstants(expr)

	// Replace mathematical functions
	expr, err := replaceFunctions(expr)
	if err != nil {
		return 0, err
	}

	// Evaluate the expression using recursive descent parser
	result, err := parseExpression(expr)
	if err != nil {
		return 0, err
	}

	return result, nil
}

// replaceConstants replaces mathematical constants with their values
func replaceConstants(expr string) string {
	replacer := strings.NewReplacer(
		"pi", fmt.Sprintf("%.15f", math.Pi),
		"e", fmt.Sprintf("%.15f", math.E),
		"π", fmt.Sprintf("%.15f", math.Pi),
	)
	return replacer.Replace(expr)
}

// replaceFunctions evaluates mathematical functions
func replaceFunctions(expr string) (string, error) {
	// Match function calls like sin(x), sqrt(x), etc.
	funcPattern := regexp.MustCompile(`(sin|cos|tan|sqrt|log|ln|abs|floor|ceil|round)\(([^()]+)\)`)

	for {
		matches := funcPattern.FindStringSubmatch(expr)
		if matches == nil {
			break
		}

		funcName := matches[1]
		argExpr := matches[2]

		// Recursively evaluate the argument
		argValue, err := parseExpression(argExpr)
		if err != nil {
			return "", fmt.Errorf("error in function %s: %w", funcName, err)
		}

		// Apply the function
		var result float64
		switch funcName {
		case "sin":
			result = math.Sin(argValue)
		case "cos":
			result = math.Cos(argValue)
		case "tan":
			result = math.Tan(argValue)
		case "sqrt":
			if argValue < 0 {
				return "", fmt.Errorf("sqrt of negative number: %f", argValue)
			}
			result = math.Sqrt(argValue)
		case "log":
			if argValue <= 0 {
				return "", fmt.Errorf("log of non-positive number: %f", argValue)
			}
			result = math.Log10(argValue)
		case "ln":
			if argValue <= 0 {
				return "", fmt.Errorf("ln of non-positive number: %f", argValue)
			}
			result = math.Log(argValue)
		case "abs":
			result = math.Abs(argValue)
		case "floor":
			result = math.Floor(argValue)
		case "ceil":
			result = math.Ceil(argValue)
		case "round":
			result = math.Round(argValue)
		default:
			return "", fmt.Errorf("unknown function: %s", funcName)
		}

		// Replace the function call with its result
		expr = strings.Replace(expr, matches[0], fmt.Sprintf("%.15f", result), 1)
	}

	return expr, nil
}

// parseExpression parses and evaluates a mathematical expression
func parseExpression(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}

	return parseAddSub(expr)
}

// parseAddSub handles addition and subtraction
func parseAddSub(expr string) (float64, error) {
	// Find the last + or - not inside parentheses
	parenDepth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		char := expr[i]
		if char == ')' {
			parenDepth++
		} else if char == '(' {
			parenDepth--
		} else if parenDepth == 0 && (char == '+' || char == '-') {
			// Don't treat leading minus as subtraction
			if i == 0 {
				continue
			}

			left, err := parseAddSub(expr[:i])
			if err != nil {
				return 0, err
			}

			right, err := parseMulDiv(expr[i+1:])
			if err != nil {
				return 0, err
			}

			if char == '+' {
				return left + right, nil
			}
			return left - right, nil
		}
	}

	return parseMulDiv(expr)
}

// parseMulDiv handles multiplication and division
func parseMulDiv(expr string) (float64, error) {
	parenDepth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		char := expr[i]
		if char == ')' {
			parenDepth++
		} else if char == '(' {
			parenDepth--
		} else if parenDepth == 0 && (char == '*' || char == '/' || char == '%') {
			left, err := parseMulDiv(expr[:i])
			if err != nil {
				return 0, err
			}

			right, err := parsePower(expr[i+1:])
			if err != nil {
				return 0, err
			}

			if char == '*' {
				return left * right, nil
			} else if char == '/' {
				if right == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return left / right, nil
			} else { // '%'
				if right == 0 {
					return 0, fmt.Errorf("modulo by zero")
				}
				return math.Mod(left, right), nil
			}
		}
	}

	return parsePower(expr)
}

// parsePower handles exponentiation
func parsePower(expr string) (float64, error) {
	parenDepth := 0
	// Right-to-left for right associativity
	for i := len(expr) - 1; i >= 0; i-- {
		char := expr[i]
		if char == ')' {
			parenDepth++
		} else if char == '(' {
			parenDepth--
		} else if parenDepth == 0 && char == '^' {
			left, err := parseUnary(expr[:i])
			if err != nil {
				return 0, err
			}

			right, err := parsePower(expr[i+1:])
			if err != nil {
				return 0, err
			}

			return math.Pow(left, right), nil
		}
	}

	return parseUnary(expr)
}

// parseUnary handles unary operators (-, +)
func parseUnary(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)

	if strings.HasPrefix(expr, "-") {
		val, err := parseUnary(expr[1:])
		if err != nil {
			return 0, err
		}
		return -val, nil
	}

	if strings.HasPrefix(expr, "+") {
		return parseUnary(expr[1:])
	}

	return parsePrimary(expr)
}

// parsePrimary handles numbers and parentheses
func parsePrimary(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)

	// Handle parentheses
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		return parseExpression(expr[1 : len(expr)-1])
	}

	// Parse number
	num, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", expr)
	}

	return num, nil
}
