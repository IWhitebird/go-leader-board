math.randomseed(os.time())

function request()
  local game_id = math.random(1, 9)
  local limit = math.random(10, 50)
  local path = string.format("/api/leaderboard/top/%d?limit=%d", game_id, limit)
  return wrk.format("GET", path)
end
