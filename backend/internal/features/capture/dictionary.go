// Purpose: External lexical dictionary client interface, HTTP adapter, and content hashing.
// Architecture: backend/internal/features/capture/dictionary.go
package capture

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrWordNotFound indicates the external dictionary does not recognize the term.
	ErrWordNotFound = errors.New("word not found in lexical dictionary")

	// ErrDictionaryUnavailable indicates network or third-party API outage.
	ErrDictionaryUnavailable = errors.New("lexical dictionary service is temporarily unavailable")
)

/// SenseDefinition encapsulates a single distinct definition, example, and synonyms.
type SenseDefinition struct {
	Definition string   `json:"definition"`
	Example    string   `json:"example,omitempty"`
	Synonyms   []string `json:"synonyms,omitempty"`
}

/// WordMeaning groups lexical senses under a specific part of speech.
type WordMeaning struct {
	PartOfSpeech string            `json:"part_of_speech"`
	Definitions  []SenseDefinition `json:"definitions"`
}

/// DefinitionResult represents the standardized payload retrieved from an external lexical provider.
type DefinitionResult struct {
	Word      string        `json:"word"`
	Phonetic  string        `json:"phonetic,omitempty"`
	Source    string        `json:"source"`
	SourceRef string        `json:"source_ref,omitempty"`
	Meanings  []WordMeaning `json:"meanings"`
	RawJSON   []byte        `json:"-"`
}

/// DictionaryClient defines the abstract contract for lexical lookups.
type DictionaryClient interface {
	FetchDefinition(ctx context.Context, headword string) (*DefinitionResult, error)
}

// -------------------------------------------------------------------------
// HTTP Dictionary Client (Free Dictionary API)
// -------------------------------------------------------------------------

type httpDictionaryClient struct {
	baseURL    string
	httpClient *http.Client
}

/// NewHTTPDictionaryClient constructs a live client querying public dictionary endpoints.
func NewHTTPDictionaryClient(timeout time.Duration) DictionaryClient {
	return &httpDictionaryClient{
		baseURL: "https://api.dictionaryapi.dev/api/v2/entries/en",
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type freeDictEntry struct {
	Word      string `json:"word"`
	Phonetic  string `json:"phonetic"`
	Meanings  []struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string   `json:"definition"`
			Example    string   `json:"example"`
			Synonyms   []string `json:"synonyms"`
		} `json:"definitions"`
	} `json:"meanings"`
}

func (c *httpDictionaryClient) FetchDefinition(ctx context.Context, headword string) (*DefinitionResult, error) {
	endpoint := fmt.Sprintf("%s/%s", c.baseURL, url.PathEscape(strings.ToLower(headword)))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("constructing dictionary request: %w", err)
	}
	req.Header.Set("User-Agent", "Wordivate-Vocabulary-App/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDictionaryUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrWordNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrDictionaryUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading dictionary response: %w", err)
	}

	var entries []freeDictEntry
	if err := json.Unmarshal(body, &entries); err != nil || len(entries) == 0 {
		return nil, fmt.Errorf("parsing dictionary payload: %w", err)
	}

	first := entries[0]
	result := &DefinitionResult{
		Word:      first.Word,
		Phonetic:  first.Phonetic,
		Source:    "free_dictionary_api",
		SourceRef: endpoint,
		RawJSON:   body,
		Meanings:  make([]WordMeaning, 0, len(first.Meanings)),
	}

	for _, m := range first.Meanings {
		meaning := WordMeaning{
			PartOfSpeech: m.PartOfSpeech,
			Definitions:  make([]SenseDefinition, 0, len(m.Definitions)),
		}
		for _, d := range m.Definitions {
			meaning.Definitions = append(meaning.Definitions, SenseDefinition{
				Definition: d.Definition,
				Example:    d.Example,
				Synonyms:   d.Synonyms,
			})
		}
		result.Meanings = append(result.Meanings, meaning)
	}

	return result, nil
}

// -------------------------------------------------------------------------
// Mock / Offline Dictionary Client (Testing & Offline Fallback)
// -------------------------------------------------------------------------

type mockDictionaryClient struct {
	entries map[string]*DefinitionResult
}

/// NewMockDictionaryClient instantiates an in-memory dictionary populated with common scholarly words.
func NewMockDictionaryClient() DictionaryClient {
	m := &mockDictionaryClient{
		entries: make(map[string]*DefinitionResult),
	}

	m.addEntry("ephemeral", "ɪˈfɛm(ə)rəl", []WordMeaning{
		{
			PartOfSpeech: "adjective",
			Definitions: []SenseDefinition{
				{
					Definition: "Lasting for a very short time; transitory; fleeting.",
					Example:    "The ephemeral joys of childhood quickly fade into adult responsibilities.",
					Synonyms:   []string{"fleeting", "short-lived", "transient", "evanescent"},
				},
				{
					Definition: "(of plants) Having a very short life cycle, typically completing growth within a few weeks.",
					Example:    "Desert ephemerals bloom rapidly after rare rains.",
				},
			},
		},
		{
			PartOfSpeech: "noun",
			Definitions: []SenseDefinition{
				{
					Definition: "An ephemeral plant or insect (such as a mayfly).",
					Example:    "The stream was teeming with ephemerals during twilight.",
				},
			},
		},
	})

	m.addEntry("serendipity", "ˌsɛr(ə)nˈdɪpɪti", []WordMeaning{
		{
			PartOfSpeech: "noun",
			Definitions: []SenseDefinition{
				{
					Definition: "The occurrence and development of events by chance in a happy or beneficial way.",
					Example:    "A fortunate stroke of serendipity led to the discovery of penicillin.",
					Synonyms:   []string{"chance", "fluke", "happy coincidence"},
				},
			},
		},
	})

	m.addEntry("ubiquitous", "juːˈbɪkwɪtəs", []WordMeaning{
		{
			PartOfSpeech: "adjective",
			Definitions: []SenseDefinition{
				{
					Definition: "Present, appearing, or found everywhere.",
					Example:    "Smartphones have become ubiquitous in modern society.",
					Synonyms:   []string{"omnipresent", "pervasive", "everywhere"},
				},
			},
		},
	})

	return m
}

func (m *mockDictionaryClient) addEntry(word, phonetic string, meanings []WordMeaning) {
	raw, _ := json.Marshal(meanings)
	m.entries[strings.ToLower(word)] = &DefinitionResult{
		Word:      word,
		Phonetic:  phonetic,
		Source:    "mock_lexical_database",
		SourceRef: "internal://mock/lexicon",
		Meanings:  meanings,
		RawJSON:   raw,
	}
}

func (m *mockDictionaryClient) FetchDefinition(ctx context.Context, headword string) (*DefinitionResult, error) {
	normalized := strings.ToLower(strings.TrimSpace(headword))
	entry, ok := m.entries[normalized]
	if !ok {
		// Provide a synthetic fallback definition for arbitrary testing words
		meanings := []WordMeaning{
			{
				PartOfSpeech: "noun",
				Definitions: []SenseDefinition{
					{
						Definition: fmt.Sprintf("A foundational lexical entry for '%s'.", normalized),
						Example:    fmt.Sprintf("Scholars frequently examine '%s' in context.", normalized),
					},
				},
			},
		}
		raw, _ := json.Marshal(meanings)
		return &DefinitionResult{
			Word:      normalized,
			Phonetic:  "",
			Source:    "synthetic_fallback",
			SourceRef: "internal://synthetic",
			Meanings:  meanings,
			RawJSON:   raw,
		}, nil
	}
	return entry, nil
}

/// ComputeContentHash generates an immutable SHA-256 byte digest for a payload.
func ComputeContentHash(payload []byte) []byte {
	hash := sha256.Sum256(payload)
	return hash[:]
}
