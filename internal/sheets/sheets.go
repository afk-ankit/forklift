package sheets

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"forklift/internal/git"
	"forklift/internal/structures"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Service struct {
	srv *sheets.Service
}

func NewService(ctx context.Context, credentialsPath string) (*Service, error) {
	if !filepath.IsAbs(credentialsPath) {
		return nil, fmt.Errorf("FORKLIFT_GOOGLE_CREDENTIALS must be an absolute path: %s", credentialsPath)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(credentialsPath), option.WithScopes(sheets.SpreadsheetsScope))
	if err != nil {
		return nil, err
	}
	return &Service{srv: srv}, nil
}

// GetRepoInfo returns a structures.RepoInfo struct for the given repository.
// If the repository is not found, it returns nil and no error.
func (s *Service) GetRepoInfo(ctx context.Context, sheetID, sheetName, repo string) (*structures.RepoInfo, error) {
	rangeName := fmt.Sprintf("%s!A:E", sheetName)
	resp, err := s.srv.Spreadsheets.Values.Get(sheetID, rangeName).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for i, row := range resp.Values {
		if len(row) < 1 {
			continue
		}
		repoName, ok := row[0].(string)
		if !ok {
			continue
		}
		if repoName == repo {
			info := &structures.RepoInfo{
				RowIdx: i,
			}
			if len(row) > 1 {
				info.MergeBranch = strings.TrimSpace(row[1].(string))
			}
			// Column D is index 3 (Repo, Branch, Time, Tag)
			if len(row) > 3 {
				info.LatestTag = strings.TrimSpace(row[3].(string))
			}
			// Column E is index 4 (User)
			if len(row) > 4 {
				info.LastUser = strings.TrimSpace(row[4].(string))
			}
			return info, nil
		}
	}

	return nil, nil
}

func (s *Service) SetMergeBranch(ctx context.Context, sheetID, sheetName, repo, branch string, rowIdx int) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	user := git.UserIdentity()

	if rowIdx >= 0 {
		// Update existing row (clearing tag for new sequence, updating user)
		values := []interface{}{branch, timestamp, "", user}
		vr := &sheets.ValueRange{Values: [][]interface{}{values}}
		rangeName := fmt.Sprintf("%s!B%d:E%d", sheetName, rowIdx+1, rowIdx+1) // Sheet is 1-indexed
		_, err := s.srv.Spreadsheets.Values.Update(sheetID, rangeName, vr).
			ValueInputOption("RAW").
			Context(ctx).
			Do()
		return err
	}

	// Append new row
	values := []interface{}{repo, branch, timestamp, "", user}
	vr := &sheets.ValueRange{Values: [][]interface{}{values}}
	_, err := s.srv.Spreadsheets.Values.Append(sheetID, fmt.Sprintf("%s!A:E", sheetName), vr).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	return err
}

func (s *Service) UpdateRepoTag(ctx context.Context, sheetID, sheetName string, rowIdx int, tag string) error {
	if rowIdx < 0 {
		return errors.New("cannot update tag for non-existent repo row")
	}
	// Update Tag (Col D), Time (Col C), and User (Col E)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	user := git.UserIdentity()
	values := []interface{}{timestamp, tag, user}
	vr := &sheets.ValueRange{Values: [][]interface{}{values}}
	rangeName := fmt.Sprintf("%s!C%d:E%d", sheetName, rowIdx+1, rowIdx+1)
	_, err := s.srv.Spreadsheets.Values.Update(sheetID, rangeName, vr).
		ValueInputOption("RAW").
		Context(ctx).
		Do()
	return err
}

func ExtractSheetID(sheetURL string) (string, error) {
	if sheetURL == "" {
		return "", errors.New("sheet URL cannot be empty")
	}
	// Supports full URLs like https://docs.google.com/spreadsheets/d/<id>/edit
	re := regexp.MustCompile(`^https?://docs\.google\.com/spreadsheets/d/([^/]+)/?`)
	matches := re.FindStringSubmatch(sheetURL)
	if len(matches) == 2 {
		return matches[1], nil
	}
	// Allow providing just the sheet ID.
	if !strings.Contains(sheetURL, "/") {
		return sheetURL, nil
	}
	return "", fmt.Errorf("unable to parse sheet id from URL: %s", sheetURL)
}
