# botany Go port - CONTRACT

Behavioral parity with the Python original. Pure logic is TDD'd; the tcell UI is
verified by building and running. Shared on-disk formats stay byte-compatible with
Python so a mixed Python/Go population on one host interoperates.

## internal/plant (TDD)

- [ ] New plant starts at stage 0 (seed), generation 1, not dead -> test
- [ ] Species/color indices within bounds; rarity in 0..4 -> test
- [ ] rarity distribution: common is most likely, godly least (256-roll bands match Python) -> test over many seeds
- [ ] life_stages thresholds = 1d,3d,10d,20d,30d in seconds -> test
- [ ] ParsePlant: stage>=3 prefixes rarity; mutation!=0 adds mutation; stage>=4 adds color; always stage; stage>=2 adds species -> test each stage
- [ ] Growth: stage increments only below final stage (5) -> test
- [ ] dead_check: >5 days since watered => dead; already-dead stays dead -> test boundary
- [ ] water_check: watered within 24h => true and watered_24h=true; else false -> test boundary
- [ ] Water(): sets watered_timestamp=now, watered_24h=true; no-op when dead -> test
- [ ] generation bonus: score_inc = 1*(1+0.2*(gen-1)); growth multiplier display 1+0.2*(gen-1) -> test
- [ ] mutate: only mutates when current mutation==0; sets a valid mutation index -> test (deterministic via injected rng)
- [ ] AgeFormat: "%dd:%dh:%dm:%ds" from start_time -> test
- [ ] StartOver: increments generation if alive, keeps if dead; resets to fresh seed -> test
- [ ] Tick logic: when alive & watered_24h, ticks += score_inc, grows at threshold -> test

## internal/storage (TDD)

- [ ] Save then Load round-trips a Plant via JSON savefile -> test
- [ ] _plant_data.json export schema matches Python keys exactly (owner, description, age, score, is_dead, last_watered, file_name, stage, generation; rarity if stage>=3; mutation if mutation!=0; color if stage>=4; species if stage>=2) -> test
- [ ] sqlite garden table created with Python schema; insert-or-replace + clean duplicate owners -> test
- [ ] retrieve_garden returns map keyed by plant_id with owner/description/age/score/dead -> test
- [ ] visitors table: insert new (weekly_visits=1) or increment existing -> test
- [ ] harvest appends plant to harvest map + json; returns new-file flag -> test
- [ ] guest watering: append {user,timestamp} to a visitors.json; respect write perms -> test
- [ ] AgeConvert matches plant age format -> test

## internal/ui (TDD where pure, else run)

- [ ] ANSI art parse: split on ESC[ , map 38;5;N codes to color pairs, plain text on .txt -> test
- [ ] water gauge string format: "()))....) NN% " style, clamps >=0 -> test
- [ ] garden sort by column (name/age/score/desc), age parsed to seconds, ascending toggle -> test
- [ ] garden filter by regex over formatted entry; invalid regex matches nothing -> test
- [ ] login completer: tab cycles prefix matches, wraps, returns base at ends -> test
- [ ] color env: PYTHON_COLORS>NO_COLOR>FORCE_COLOR precedence -> test
- [ ] (run) menu navigation, live plant growth redraw, look/instructions/garden/visit panes, harvest flow

## main / integration (run)

- [ ] go build ./... clean, go vet clean
- [ ] go test ./... all green
- [ ] Running the binary shows the menu, plant art, water gauge; watering works; quitting saves
