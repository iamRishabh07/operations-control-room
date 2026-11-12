.PHONY: acquire acquire-monthly test-acquire
acquire:
	python3 scripts/acquire.py
acquire-monthly:
	python3 scripts/acquire.py --monthly
test-acquire:
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts -p 'test_*.py' -v
# setup/run are intentionally not implemented until the source-feasibility gate passes.
