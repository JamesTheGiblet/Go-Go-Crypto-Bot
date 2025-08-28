package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CompileRequest struct {
	Code string `json:"code"`
}

type CompileResponse struct {
	Success bool   `json:"success"`
	URL     string `json:"url,omitempty"`
	Error   string `json:"error,omitempty"`
}

// compileAndBuild handles reading the template, injecting user code, and compiling.
func compileAndBuild(userCode string) (string, error) {
	// 1. Read the template main.go
	templateBytes, err := ioutil.ReadFile("main.go")
	if err != nil {
		return "", fmt.Errorf("failed to read main.go template: %w", err)
	}
	templateCode := string(templateBytes)

	// 2. Inject user code into the placeholders
	finalCode := strings.Replace(templateCode, "// [[USER_MOD_STRATEGIES]]", userCode, 1)
	registration := `"user_mod": strategyUserMod, /* [[USER_MOD_REGISTRATION]] */`
	finalCode = strings.Replace(finalCode, "/* [[USER_MOD_REGISTRATION]] */", registration, 1)

	// 3. Create a temporary directory for the build to keep things clean
	buildDir, err := ioutil.TempDir("", "ganymede-build-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp build directory: %w", err)
	}
	defer os.RemoveAll(buildDir) // Clean up afterward

	// 4. Write the new source code to the temp directory
	if err := ioutil.WriteFile(filepath.Join(buildDir, "main.go"), []byte(finalCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp go file: %w", err)
	}

	// 5. Compile the code, placing the output in a 'public' directory
	os.MkdirAll("public", 0755) // Ensure the public directory exists
	wasmFile := fmt.Sprintf("mod_%d.wasm", time.Now().UnixNano())
	wasmPath := filepath.Join("public", wasmFile)

	cmd := exec.Command("go", "build", "-o", wasmPath, "main.go")
	cmd.Dir = buildDir // Run the command in the temp directory
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("compilation failed: %s", stderr.String())
	}

	// 6. Return the URL to the new WASM file
	return "/" + wasmPath, nil
}

// validateCode checks if the user's code compiles without creating a permanent file.
func validateCode(userCode string) (string, error) {
	templateBytes, err := ioutil.ReadFile("main.go")
	if err != nil {
		return "", fmt.Errorf("failed to read main.go template: %w", err)
	}
	templateCode := string(templateBytes)

	finalCode := strings.Replace(templateCode, "// [[USER_MOD_STRATEGIES]]", userCode, 1)
	registration := `"user_mod": strategyUserMod, /* [[USER_MOD_REGISTRATION]] */`
	finalCode = strings.Replace(finalCode, "/* [[USER_MOD_REGISTRATION]] */", registration, 1)

	buildDir, err := ioutil.TempDir("", "ganymede-validate-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp validation directory: %w", err)
	}
	defer os.RemoveAll(buildDir)

	if err := ioutil.WriteFile(filepath.Join(buildDir, "main.go"), []byte(finalCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp go file: %w", err)
	}

	// We compile to a dummy output path within the temp directory.
	cmd := exec.Command("go", "build", "-o", filepath.Join(buildDir, "output.wasm"), "main.go")
	cmd.Dir = buildDir
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return stderr.String(), cmd.Run()
}

func compileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Println("Received compilation request...")
	wasmURL, err := compileAndBuild(req.Code)
	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		log.Printf("Compilation error: %v", err)
		json.NewEncoder(w).Encode(CompileResponse{Success: false, Error: err.Error()})
		return
	}

	log.Printf("Compilation successful. New WASM at: %s", wasmURL)
	json.NewEncoder(w).Encode(CompileResponse{Success: true, URL: wasmURL})
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Println("Received validation request...")
	compilerOutput, err := validateCode(req.Code)
	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		log.Printf("Validation failed:\n%s", compilerOutput)
		json.NewEncoder(w).Encode(CompileResponse{Success: false, Error: compilerOutput})
		return
	}

	log.Println("Validation successful.")
	json.NewEncoder(w).Encode(CompileResponse{Success: true})
}

func main() {
	// This server will handle API calls and serve static files.
	mux := http.NewServeMux()
	mux.HandleFunc("/compile", compileHandler)
	mux.HandleFunc("/validate", validateHandler)
	mux.Handle("/", http.FileServer(http.Dir("."))) // Serves all project files

	log.Println("Starting server on :8080...")
	log.Println("Visit http://localhost:8080/crypto-bot.html to use the application.")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}