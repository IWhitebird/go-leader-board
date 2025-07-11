// Package docs Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/api/health": {
            "get": {
                "description": "Returns the current status of the API",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "health"
                ],
                "summary": "Health check endpoint",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/models.HealthResponse"
                        }
                    }
                }
            }
        },
        "/api/leaderboard/rank/{gameId}/{userId}": {
            "get": {
                "description": "Returns the rank and percentile for a specific player in a game",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "leaderboard"
                ],
                "summary": "Get a player's rank",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Game ID",
                        "name": "gameId",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "integer",
                        "description": "User ID",
                        "name": "userId",
                        "in": "path",
                        "required": true
                    },
                    {
                        "enum": [
                            "24h",
                            "3d",
                            "7d"
                        ],
                        "type": "string",
                        "description": "Time window (empty for all-time, 24h for last 24 hours, 3d for 3 days, 7d for 7 days)",
                        "name": "window",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/models.PlayerRankResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/api/leaderboard/score": {
            "post": {
                "description": "Records a new score for a player in a game",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "leaderboard"
                ],
                "summary": "Submit a player's score",
                "parameters": [
                    {
                        "description": "Score data",
                        "name": "score",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/models.Score"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK"
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/api/leaderboard/top/{gameId}": {
            "get": {
                "description": "Returns the top scoring players for a specific game",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "leaderboard"
                ],
                "summary": "Get top leaders for a game",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Game ID",
                        "name": "gameId",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "integer",
                        "default": 10,
                        "description": "Number of leaders to return",
                        "name": "limit",
                        "in": "query"
                    },
                    {
                        "enum": [
                            "24h",
                            "3d",
                            "7d"
                        ],
                        "type": "string",
                        "description": "Time window (empty for all-time, 24h for last 24 hours, 3d for 3 days, 7d for 7 days)",
                        "name": "window",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/models.TopLeadersResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "models.HealthResponse": {
            "type": "object",
            "properties": {
                "status": {
                    "type": "string"
                },
                "timestamp": {
                    "type": "string"
                },
                "version": {
                    "type": "string"
                }
            }
        },
        "models.LeaderboardEntry": {
            "type": "object",
            "properties": {
                "rank": {
                    "type": "integer"
                },
                "score": {
                    "type": "integer"
                },
                "user_id": {
                    "type": "integer"
                }
            }
        },
        "models.PlayerRankResponse": {
            "type": "object",
            "properties": {
                "game_id": {
                    "type": "integer"
                },
                "percentile": {
                    "type": "number"
                },
                "rank": {
                    "type": "integer"
                },
                "score": {
                    "type": "integer"
                },
                "total_players": {
                    "type": "integer"
                },
                "user_id": {
                    "type": "integer"
                },
                "window": {
                    "type": "string"
                }
            }
        },
        "models.Score": {
            "type": "object",
            "properties": {
                "game_id": {
                    "type": "integer"
                },
                "score": {
                    "type": "integer"
                },
                "timestamp": {
                    "type": "string"
                },
                "user_id": {
                    "type": "integer"
                }
            }
        },
        "models.TopLeadersResponse": {
            "type": "object",
            "properties": {
                "game_id": {
                    "type": "integer"
                },
                "leaders": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/models.LeaderboardEntry"
                    }
                },
                "total_players": {
                    "type": "integer"
                },
                "window": {
                    "type": "string"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "",
	Description:      "",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
