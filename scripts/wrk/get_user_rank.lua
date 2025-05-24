math.randomseed(os.time())

function request()
  local game_id = math.random(1, 50)
  local user_id = math.random(1, 1000000000)
  local path = string.format("/api/leaderboard/rank/%d/%d", game_id, user_id)
  return wrk.format("GET", path)
end
