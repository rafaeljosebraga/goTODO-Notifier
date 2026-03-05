package anytype

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gen2brain/beeep"
)

const baseURL = "http://127.0.0.1:31012/v1"

type Client struct {
	HTTPClient *http.Client
	APIKey     string
}

func NewClient(apiKey string) *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		APIKey:     apiKey,
	}
}

func (c *Client) Notify(title, message string) error {
	return beeep.Notify(title, message, "")
}

func (c *Client) makeRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.APIKey)
	req.Header.Add("Anytype-Version", "2024-01-01")

	res, err := c.HTTPClient.Do(req)
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

func (c *Client) GetFirstSpaceID() (string, string, error) {
	body, err := c.makeRequest(baseURL + "/spaces")
	if err != nil {
		return "", "", err
	}

	var response struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", "", err
	}

	if len(response.Data) == 0 {
		return "", "", fmt.Errorf("no spaces found")
	}

	for _, s := range response.Data {
		if s.Name == "Faculdade" {
			return s.ID, s.Name, nil
		}
	}
	return response.Data[0].ID, response.Data[0].Name, nil
}

func (c *Client) DiscoverTaskTypeID(spaceID string) (string, error) {
	url := fmt.Sprintf("%s/spaces/%s/types", baseURL, spaceID)
	body, err := c.makeRequest(url)
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

type Task struct {
	ID          string
	Name        string
	DueDate     time.Time
	Status      string
	IsCompleted bool
}

func (c *Client) FetchTasks(spaceID, typeID string) ([]Task, error) {
	url := fmt.Sprintf("%s/spaces/%s/objects", baseURL, spaceID)
	body, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var rawResponse struct {
		Data []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Layout     string `json:"layout"`
			Properties []struct {
				Key      string `json:"key"`
				Date     string `json:"date"`
				Checkbox *bool  `json:"checkbox"`
				Select   *struct {
					Name string `json:"name"`
				} `json:"select"`
			} `json:"properties"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, err
	}

	var tasks []Task
	for _, obj := range rawResponse.Data {
		if obj.Layout == "action" || obj.Layout == "task" {
			t := Task{ID: obj.ID, Name: obj.Name}
			// Parse properties
			for _, p := range obj.Properties {
				switch p.Key {
				case "due_date":
					if p.Date != "" {
						parsedDate, err := time.Parse(time.RFC3339, p.Date)
						if err != nil {
							parsedDate, err = time.Parse("2006-01-02", p.Date)
						}
						if err == nil {
							t.DueDate = parsedDate
						}
					}
				case "status":
					if p.Select != nil {
						t.Status = p.Select.Name
					}
				case "done":
					if p.Checkbox != nil {
						t.IsCompleted = *p.Checkbox
					}
				}
			}
			tasks = append(tasks, t)
		}
	}

	return tasks, nil
}
