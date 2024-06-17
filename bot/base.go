package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type Whitelist struct {
	ExclusiveJoin []string `json:"ExclusiveJoin"`
}

var commands = []*discordgo.ApplicationCommand{
    {
        Type:                     discordgo.ChatApplicationCommand,
        Name:                     "replacefile",
        DefaultMemberPermissions: Ptr(int64(discordgo.PermissionAdministrator)),
        DMPermission:             new(bool),
        NSFW:                     new(bool),
        Description:              "Replace the contents of one file with another",
        Options: []*discordgo.ApplicationCommandOption{
            {
                Type:         discordgo.ApplicationCommandOptionString,
                Name:         "source_folder",
                Description:  "The folder containing the source file.",
                Required:     true,
                Autocomplete: true,
            },
            {
                Type:         discordgo.ApplicationCommandOptionString,
                Name:         "source_file",
                Description:  "The source file to copy from.",
                Required:     true,
                Autocomplete: true,
            },
            {
                Type:         discordgo.ApplicationCommandOptionString,
                Name:         "target_folder",
                Description:  "The folder containing the target file.",
                Required:     true,
                Autocomplete: true,
            },
            {
                Type:         discordgo.ApplicationCommandOptionString,
                Name:         "target_file",
                Description:  "The target file to replace.",
                Required:     true,
                Autocomplete: true,
            },
        },
    },

}

var autoCompleteFile = map[string][]string{}

func BotConnect(token string) (*discordgo.Session, error) {

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return s, fmt.Errorf("Discordgo.New Error: %w", err)
	}

	s.Identify.Intents = discordgo.IntentsAllWithoutPrivileged
	s.AddHandler(HandleCommands)

	err = s.Open()
	if err != nil {
		return s, fmt.Errorf("failed to open a websocket connection with discord. Likely due to an invalid token. %w", err)
	}

	go updateFolders()

	_, err = s.ApplicationCommandBulkOverwrite(s.State.User.ID, "", commands)
	if err != nil {
		return s, fmt.Errorf("failed to register commands: %w", err)
	}

	return s, nil
}

func FetchRepo() (*git.Repository, string, error) {
	tmpDir, err := os.MkdirTemp("", "whitelist")
	if err != nil {
		fmt.Println("Error creating temp directory:", err)
		return nil, "", err
	}

	repo, err := git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:      os.Getenv("GITHUB_URL"),
		Progress: os.Stdout,
	})
	if err != nil {
		fmt.Println("Error cloning repository:", err)
		return repo, "", err
	}

	return repo, tmpDir, nil
}

// Function to get the whitelist for a file
func ReplaceFileInRepo(sourceFolder, sourceFile, targetFolder, targetFile string) error {
    // Clone the repository to a temporary directory
    repo, tmpDir, err := FetchRepo()
    defer os.RemoveAll(tmpDir)
    if err != nil {
        return err
    }

    // Read the source file content
    sourceFilePath := filepath.Join(tmpDir, sourceFolder, sourceFile)
    sourceContent, err := os.ReadFile(sourceFilePath)
    if err != nil {
        return err
    }

    // Write the source content to the target file
    targetFilePath := filepath.Join(tmpDir, targetFolder, targetFile)
    if err := os.WriteFile(targetFilePath, sourceContent, 0644); err != nil {
        return err
    }

    // Git operations: add, commit, and push
    worktree, err := repo.Worktree()
    if err != nil {
        return err
    }

    _, err = worktree.Add(".")
    if err != nil {
        return err
    }

    _, err = worktree.Commit("Replace file content", &git.CommitOptions{
        Author: &object.Signature{
            Name:  "Ark Whitelist Bot",
            Email: "",
            When:  time.Now(),
        },
    })
    if err != nil {
        return err
    }

    auth := &http.BasicAuth{
        Username: os.Getenv("GITHUB_USERNAME"), // Replace with your GitHub username
        Password: os.Getenv("GITHUB_TOKEN"),    // Replace with your GitHub token
    }

    err = repo.Push(&git.PushOptions{
        Auth: auth,
    })
    if err != nil {
        return err
    }

    return nil
}


func UpdateRepo(folderName, fileName string, whitelist *Whitelist) error {
	// Clone the repository to a temporary directory

	repo, tmpDir, err := FetchRepo()
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return err
	}

	// Marshal the whitelist into JSON
	whitelistContent, err := json.MarshalIndent(whitelist, "", "    ")
	if err != nil {
		return err
	}

	fmt.Println(filepath.Join(tmpDir, folderName, fileName))

	if err := os.WriteFile(filepath.Join(tmpDir, folderName, fileName), whitelistContent, 0644); err != nil {
		return err
	}

	// Git operations: add, commit, and push
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	_, err = worktree.Add(".")
	if err != nil {
		return err
	}

	_, err = worktree.Commit("Update whitelist", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Ark Whitelist Bot",
			Email: "",
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	auth := &http.BasicAuth{
		Username: os.Getenv("GITHUB_USERNAME"), // Replace with your GitHub username
		Password: os.Getenv("GITHUB_TOKEN"),    // Replace with your GitHub token
	}

	err = repo.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		return err
	}

	return nil
}

func ParseSlashCommand(i *discordgo.InteractionCreate) map[string]interface{} {
	var options = make(map[string]interface{})
	for _, option := range i.ApplicationCommandData().Options {
		options[option.Name] = option.Value
	}

	return options
}

func Ptr[T any](v T) *T {
	return &v
}

func HandleCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommand {
		switch i.ApplicationCommandData().Name {
		case "replacefile":
            ReplaceFileCommand(s, i)
		}
	} else if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
		switch i.ApplicationCommandData().Name {
		case "replacefile":
            ReplaceFileCommand(s, i)
		}
	}

}

func HandleCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
    if i.Type == discordgo.InteractionApplicationCommand {
        switch i.ApplicationCommandData().Name {
        case "whitelist":
            WhitelistCommand(s, i)
        case "replacefile":
            ReplaceFileCommand(s, i)
        }
    } else if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
        switch i.ApplicationCommandData().Name {
        case "whitelist":
            WhitelistAutoComplete(s, i)
        case "replacefile":
            ReplaceFileAutoComplete(s, i)
        }
    }
}


func WhitelistAutoComplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Iterate through all options to find the focused one
	var focusedOption *discordgo.ApplicationCommandInteractionDataOption
	for _, option := range i.ApplicationCommandData().Options {
		if option.Focused {
			focusedOption = option
			break
		}
	}

	// Check if a focused option was found and process accordingly
	if focusedOption != nil {
		switch focusedOption.Name {
		case "folder":
			choices := []*discordgo.ApplicationCommandOptionChoice{}
			for folder := range autoCompleteFile {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  folder,
					Value: folder,
				})
			}

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionApplicationCommandAutocompleteResult,
				Data: &discordgo.InteractionResponseData{
					Choices: choices,
				},
			})
			if err != nil {
				fmt.Println("Error sending autocomplete response:", err)
			}
			return

        case "file", "source_file", "target_file":
			options := ParseSlashCommand(i)
			if options["folder"] == nil {
				fmt.Println("No folder provided")
				err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionApplicationCommandAutocompleteResult,
					Data: &discordgo.InteractionResponseData{
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "Please select a folder first.",
								Value: "",
							},
						},
					},
				})
				if err != nil {
					fmt.Println("Error sending autocomplete response:", err)
				}
				return
			}

			response := []*discordgo.ApplicationCommandOptionChoice{}
			for _, file := range autoCompleteFile[options["folder"].(string)] {
				response = append(response, &discordgo.ApplicationCommandOptionChoice{
					Name:  file,
					Value: file,
				})
			}

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionApplicationCommandAutocompleteResult,
				Data: &discordgo.InteractionResponseData{
					Choices: response,
				},
			})
			if err != nil {
				fmt.Println("Error sending autocomplete response:", err)
			}
		}
	} else {
		// Respond with a generic error or guidance message if no option is focused
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Please select an option.",
						Value: "",
					},
				},
			},
		})
		if err != nil {
			fmt.Println("Error sending autocomplete response:", err)
		}
	}
}

// Function to update the autocomplete list, runs every 30 minutes
func updateFolders() {
	for {
		_, tmpDir, err := FetchRepo()
		if err != nil {
			fmt.Println("Error fetching repo:", err)
			return
		}

		defer os.RemoveAll(tmpDir)

		files, err := os.ReadDir(tmpDir)
		if err != nil {
			fmt.Println("Error reading directory:", err)
		}

		for _, folder := range files {
			if folder.Name() == ".git" || !folder.IsDir() {
				continue
			}

			dirPath := filepath.Join(tmpDir, folder.Name())
			subFiles, err := os.ReadDir(dirPath)
			if err != nil {
				fmt.Println("Error reading directory:", err)
				continue
			}

			for _, subFile := range subFiles {
				if !sliceContains(autoCompleteFile[folder.Name()], subFile.Name()) {
					autoCompleteFile[folder.Name()] = append(autoCompleteFile[folder.Name()], subFile.Name())
				}
			}

		}

		time.Sleep(30 * time.Minute)
	}
}

// Helper function to check if a slice contains a specific string.
func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
