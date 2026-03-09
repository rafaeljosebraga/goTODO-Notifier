package anytype

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gen2brain/beeep"
)

const defaultBaseURL = "http://127.0.0.1:31012/v1"

type Notifier interface {
	Notify(title, message string) error
}

type Client struct {
	HTTPClient *http.Client
	APIKey     string
	BaseURL    string
}

func NewClient(apiKey string) *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
	}
}

func (c *Client) Notify(title, message string) error {
	err := beeep.Notify(title, message, "")
	if err != nil {
		slog.Error("failed to send notification", "title", title, "error", err)
	}
	return err
}

func (c *Client) makeRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("failed to create request", "url", url, "error", err)
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.APIKey)
	req.Header.Add("Anytype-Version", "2024-01-01")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		slog.Error("HTTP request failed", "url", url, "error", err)
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		slog.Error("API returned error status", "url", url, "status", res.StatusCode, "body", string(body))
		return nil, fmt.Errorf("API failed (%d): %s", res.StatusCode, string(body))
	}

	return io.ReadAll(res.Body)
}

func (c *Client) GetFirstSpaceID() (string, string, error) {
	body, err := c.makeRequest(c.BaseURL + "/spaces")
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch spaces: %w", err)
	}

	var response struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		slog.Error("failed to unmarshal spaces response", "error", err)
		return "", "", fmt.Errorf("failed to decode spaces: %w", err)
	}

	if len(response.Data) == 0 {
		slog.Warn("no spaces found in Anytype")
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
	url := fmt.Sprintf("%s/spaces/%s/types", c.BaseURL, spaceID)
	body, err := c.makeRequest(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch types for space %s: %w", spaceID, err)
	}

	var response struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		slog.Error("failed to unmarshal types response", "space_id", spaceID, "error", err)
		return "", fmt.Errorf("failed to decode types: %w", err)
	}

	for _, t := range response.Data {
		if t.Name == "Task" || t.Name == "Tarefa" {
			return t.ID, nil
		}
	}
	slog.Warn("task type not found in space", "space_id", spaceID)
	return "", fmt.Errorf("task type not found in space")
}

type SubTask struct {
	Name        string
	IsCompleted bool
}

func ParseSubTasks(markdown string) []SubTask {
	var subTasks []SubTask
	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") {
			subTasks = append(subTasks, SubTask{
				Name:        strings.TrimPrefix(trimmed, "- [ ] "),
				IsCompleted: false,
			})
		} else if strings.HasPrefix(trimmed, "- [x] ") {
			subTasks = append(subTasks, SubTask{
				Name:        strings.TrimPrefix(trimmed, "- [x] "),
				IsCompleted: true,
			})
		}
	}
	return subTasks
}

func CleanMarkdown(md string) string {
	// Pattern to match: [Task Name](anytype://object?id=...)
	// We want to keep: Task Name
	re := regexp.MustCompile(`\[([^\]]+)\]\(anytype://object\?id=[^\)]+\)`)
	return re.ReplaceAllString(md, "$1")
}

type Task struct {
	ID          string
	Name        string
	DueDate     time.Time
	Status      string
	IsCompleted bool
	Links       []string
	LinkNames   map[string]string
	Markdown    string
}

func (c *Client) ResolveTaskName(spaceID, objectID string) (string, error) {
	url := fmt.Sprintf("%s/spaces/%s/objects/%s", c.BaseURL, spaceID, objectID)
	body, err := c.makeRequest(url)

	if err != nil {
		return "", fmt.Errorf("failed to fetch name for object %s: %w", objectID, err)
	}

	var response struct {
		Object struct {
			Name string `json:"name"`
		} `json:"object"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		slog.Error("failed to unmarshal object response", "object_id", objectID, "error", err)
		return "", fmt.Errorf("failed to decode object name: %w", err)
	}

	return response.Object.Name, nil
}

func (c *Client) FetchTasks(spaceID, typeID string) ([]Task, error) {
	url := fmt.Sprintf("%s/spaces/%s/objects", c.BaseURL, spaceID)
	body, err := c.makeRequest(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch objects for space %s: %w", spaceID, err)
	}

	var rawResponse struct {
		Data []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Layout     string `json:"layout"`
			Properties []struct {
				Key      string   `json:"key"`
				Date     string   `json:"date"`
				Checkbox *bool    `json:"checkbox"`
				Objects  []string `json:"objects"`
				Select   *struct {
					Name string `json:"name"`
				} `json:"select"`
			} `json:"properties"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		slog.Error("failed to unmarshal tasks response", "space_id", spaceID, "error", err)
		return nil, fmt.Errorf("failed to decode tasks: %w", err)
	}

	var tasks []Task
	for _, obj := range rawResponse.Data {
		if obj.Layout == "action" || obj.Layout == "task" {
			t := Task{
				ID:        obj.ID,
				Name:      obj.Name,
				LinkNames: make(map[string]string),
			}
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
						} else {
							slog.Warn("failed to parse due date", "task_id", obj.ID, "date", p.Date, "error", err)
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
				case "links":
					t.Links = p.Objects
				}
			}
			tasks = append(tasks, t)
		}
	}

	return tasks, nil
}

func (c *Client) FetchObjectDetails(spaceID, objectID string) (string, error) {
	url := fmt.Sprintf("%s/spaces/%s/objects/%s", c.BaseURL, spaceID, objectID)
	body, err := c.makeRequest(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch details for object %s: %w", objectID, err)
	}

	var response struct {
		Object struct {
			Markdown string `json:"markdown"`
		} `json:"object"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		slog.Error("failed to unmarshal object details", "object_id", objectID, "error", err)
		return "", fmt.Errorf("failed to decode object details: %w", err)
	}

	return response.Object.Markdown, nil
}
