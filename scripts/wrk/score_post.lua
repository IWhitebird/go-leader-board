-- Configuration variables
local CONFIG = {
  min_game_id = 1,
  max_game_id = 50,
  min_user_id = 1,
  max_user_id = 1000000000,
  min_score = 100,
  max_score = 1000000
}

wrk.method = "POST"
wrk.headers["Content-Type"] = "application/json"

-- Generate a random number between min and max
math.randomseed(os.time())

function request()
  local game_id = math.random(CONFIG.min_game_id, CONFIG.max_game_id)
  local user_id = math.random(CONFIG.min_user_id, CONFIG.max_user_id)
  local score = math.random(CONFIG.min_score, CONFIG.max_score)
  local timestamp = os.date("!%Y-%m-%dT%H:%M:%SZ")

  local body = string.format('{"game_id":%d,"user_id":%d,"score":%d,"timestamp":"%s"}',
                              game_id, user_id, score, timestamp)

  wrk.body = body
  return wrk.format(nil, "/api/leaderboard/score")
end
