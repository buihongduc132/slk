# Ralph Loop Context

## Context added at 2026-09-27T18:02:10.706Z
Housekeeping note, sent at launch.

If you create a scratch/debug test file to instrument behaviour, that is fine and encouraged — but DELETE it before you finish. Commit c49bace on this branch's history had to strip internal/fuzzy/fuzzy_scratch_test.go (a TestScratch that only t.Logf'd two Match calls) plus 8 .orig/.rej pairs and 18 patch_*.py scripts left behind by a previous attempt. Do not recreate that mess.

Use the Edit tool directly. Do NOT generate patch_*.py scripts to apply changes: the previous attempt left the tree uncompilable with a duplicate skipClamp field because a patch script was applied twice.

The gate does not check for stray files, so this is on your honour.
