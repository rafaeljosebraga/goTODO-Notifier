package main

import (
	"encoding/json"
	"fmt"
	"goTODO/config"
	"io"
	"log"
	"net/http"
	"time"
)

const baseURL = "http://127.0.0.1:31012/v1"

func main() {
	cfg := config.Load()

	log.Println("Starting goTODO with Anytype CLI (Port 31012)...")

	client := &http.Client{Timeout: 10 * time.Second}
	apiKey := cfg.APIKey

	// 1. Get Space ID
	spaceID, err := getFirstSpaceID(client, apiKey)
	if err != nil {
		log.Fatalf("Failed to get space ID: %v", err)
	}
	fmt.Printf("Using Space ID: %s\n", spaceID)

	// 2. Discover Task Type ID
	taskTypeID, err := discoverTaskTypeID(client, apiKey, spaceID)
	if err != nil {
		log.Printf("Warning: Could not discover Task type ID dynamically: %v. Using fallback 'ot-task'", err)
		taskTypeID = "ot-task"
	}
	fmt.Printf("Using Task Type ID: %s\n", taskTypeID)

	// 3. Fetch Tasks
	err = fetchAndPrintTasks(client, apiKey, spaceID, taskTypeID)
	if err != nil {
		log.Fatalf("Failed to fetch tasks: %v", err)
	}
}

func makeRequest(client *http.Client, apiKey, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("Anytype-Version", "2024-01-01")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("API failed (%d): %s", res.StatusCode, string(body))
	}

	return io.ReadAll(res.Body)
}

func getFirstSpaceID(client *http.Client, apiKey string) (string, error) {
	body, err := makeRequest(client, apiKey, baseURL+"/spaces")
	if err != nil {
		return "", err
	}

	var response struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	for _, s := range response.Data {
		if s.Name == "Faculdade" {
			return s.ID, nil
		}
	}
	return response.Data[0].ID, nil
}

func discoverTaskTypeID(client *http.Client, apiKey, spaceID string) (string, error) {
	url := fmt.Sprintf("%s/spaces/%s/types", baseURL, spaceID)
	body, err := makeRequest(client, apiKey, url)
	if err != nil {
		return "", err
	}

	var response struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	for _, t := range response.Data {
		if t.Name == "Task" || t.Name == "Tarefa" {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("task type not found in space")
}

func fetchAndPrintTasks(client *http.Client, apiKey, spaceID, typeID string) error {
	// Trying with the discovered type ID
	url := fmt.Sprintf("%s/spaces/%s/objects?type=%s", baseURL, spaceID, typeID)
	body, err := makeRequest(client, apiKey, url)
	if err != nil {
		// If filtering still fails, fetch all and filter manually in Go
		log.Printf("Filtering by type failed, fetching all objects and filtering manually...")
		url = fmt.Sprintf("%s/spaces/%s/objects", baseURL, spaceID)
		body, err = makeRequest(client, apiKey, url)
		if err != nil {
			return err
		}
	}

	var rawResponse struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return err
	}

	fmt.Println("Tasks found:")
	for _, obj := range rawResponse.Data {
		// Manual filter: check if it's a task by layout or type if available
		layout, _ := obj["layout"].(string)
		name, _ := obj["name"].(string)
		
		if layout == "action" || layout == "task" {
			fmt.Printf("- [ ] %s (ID: %s)\n", name, obj["id"])
		}
	}

	return nil
}
