package h3

import "testing"

func TestPooledGridDiskDistances(t *testing.T) {
	t.Run("no pentagon", func(t *testing.T) {
		p := NewPoolDiskCreator()
		rings, closer := p.GridDiskDistances(validCell, len(validDiskDist3_1)-1)
		defer closer()

		assertEqualDiskDistances(t, validDiskDist3_1, rings)
	})
	t.Run("pentagon centered", func(t *testing.T) {
		assertNoPanic(t, func() {
			p := NewPoolDiskCreator()
			rings, closer := p.GridDiskDistances(pentagonCell, 1)
			defer closer()

			assertEqual(t, 2, len(rings), "expected 2 rings")
			assertEqual(t, 5, len(rings[1]), "expected 5 cells in second ring")
		})
	})
}
