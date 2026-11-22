# WordSeek added to WasteMusic

The WordSeek word-game system has been ported into WasteMusic's Go/Meow bot and uses the existing MongoDB connection/database.

Commands added:
- `/new4` — start a 4-letter game
- `/new5` — start a 5-letter game
- `/new6` — start a 6-letter game
- `/leaderboard` — view the leaderboard; optional filters: `global|group`, `today|week|month|year|all`, and `4|5|6`

Guesses are sent as ordinary chat messages. Feedback uses 🟩 (correct position), 🟨 (in word), and 🟥 (not in word). Games allow 30 guesses. The first player to solve earns `30 - previous wrong guesses` points. Leaderboard buttons support Global, refresh, This chat, Today, This week, This month, This year, All time, and 4/5/6 letters. Updates, Donate, and Discussion buttons are intentionally omitted.

Game data is stored in MongoDB collections `wordseek_games`, `wordseek_guesses`, and `wordseek_scores`; existing music collections are not changed. Word lists are bundled from the WordSeek project.

Build validation could not be run in this environment because the source requires Go 1.26 and the available Go toolchain is 1.23; downloading the required toolchain was blocked by unavailable network access.
