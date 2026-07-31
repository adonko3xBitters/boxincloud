package discovery

import (
	"context"
	"errors"
	"testing"
	"time"
)

/*
Limitation sortante et cache, sur une horloge fictive.

Attendre réellement ferait durer ces tests exactement ce qu'ils mesurent, et
rendrait le premier d'entre eux instable sur une machine chargée. L'horloge et
le sommeil sont donc remplacés : ce qui est vérifié n'est pas qu'on a dormi,
c'est COMBIEN on aurait dormi.
*/

// fakeClock avance à la demande, et note ce qu'on lui a demandé d'attendre.
type fakeClock struct {
	now    time.Time
	slept  []time.Duration
	cancel bool
}

func (c *fakeClock) sleep(_ context.Context, d time.Duration) error {
	if c.cancel {
		return context.Canceled
	}
	c.slept = append(c.slept, d)
	c.now = c.now.Add(d)
	return nil
}

func newTestThrottle() (*Throttle, *fakeClock) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	throttle := NewThrottle()
	throttle.now = func() time.Time { return clock.now }
	throttle.sleep = clock.sleep
	return throttle, clock
}

// TestThrottleAllowsBurstThenSpaces vérifie la forme du débit.
//
// Une rafale courte passe — trois recherches lancées de front sont un usage
// normal, et les espacer rendrait l'interface poussive sans rien protéger. Au
// delà, l'intervalle s'applique.
func TestThrottleAllowsBurstThenSpaces(t *testing.T) {
	throttle, clock := newTestThrottle()
	throttle.SetRate("openlibrary", Rate{Every: time.Second, Burst: 3})

	for i := 0; i < 3; i++ {
		if err := throttle.Wait(context.Background(), "openlibrary"); err != nil {
			t.Fatalf("requête %d : %v", i, err)
		}
	}
	if len(clock.slept) != 0 {
		t.Errorf("la rafale a été retardée : %v", clock.slept)
	}

	if err := throttle.Wait(context.Background(), "openlibrary"); err != nil {
		t.Fatal(err)
	}
	if len(clock.slept) != 1 {
		t.Fatalf("%d attentes, attendu 1", len(clock.slept))
	}
	// Le seau est vide : il faut un intervalle entier pour un jeton.
	if clock.slept[0] < 900*time.Millisecond || clock.slept[0] > time.Second {
		t.Errorf("attente = %v, attendue proche d'une seconde", clock.slept[0])
	}
}

// TestThrottleRefillsOverTime : le seau se recharge continûment.
//
// Au prorata plutôt que par paliers : une requête arrivée juste avant un palier
// attendrait sinon un intervalle entier pour rien.
func TestThrottleRefillsOverTime(t *testing.T) {
	throttle, clock := newTestThrottle()
	throttle.SetRate("ia", Rate{Every: time.Second, Burst: 1})

	if err := throttle.Wait(context.Background(), "ia"); err != nil {
		t.Fatal(err)
	}

	// Trois quarts d'intervalle passent d'eux-mêmes : il ne doit rester qu'un
	// quart à attendre, pas une seconde entière.
	clock.now = clock.now.Add(750 * time.Millisecond)

	if err := throttle.Wait(context.Background(), "ia"); err != nil {
		t.Fatal(err)
	}
	if len(clock.slept) != 1 {
		t.Fatalf("%d attentes, attendu 1", len(clock.slept))
	}
	if clock.slept[0] > 400*time.Millisecond {
		t.Errorf("attente = %v : le seau ne se recharge pas au prorata", clock.slept[0])
	}
}

// TestThrottleCompartmentsAreIndependent : un catalogue lent ne doit pas
// retenir les autres.
//
// C'est ce qui permet de garder un débit prudent vers une base publique sans
// brider le Komga de l'utilisateur, qui tourne sur sa propre machine.
func TestThrottleCompartmentsAreIndependent(t *testing.T) {
	throttle, clock := newTestThrottle()
	throttle.SetRate("openlibrary", Rate{Every: time.Second, Burst: 1})
	throttle.SetRate("opds", Rate{Every: time.Millisecond, Burst: 10})

	if err := throttle.Wait(context.Background(), "openlibrary"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		if err := throttle.Wait(context.Background(), "opds"); err != nil {
			t.Fatal(err)
		}
	}
	if len(clock.slept) != 0 {
		t.Errorf("le compartiment OPDS a été retardé par l'autre : %v", clock.slept)
	}
}

// TestThrottleUnknownKindIsNotBlocked : un fournisseur sans débit déclaré passe.
//
// Un oubli de configuration doit rendre le trafic trop rapide, pas bloquer une
// fonctionnalité entière.
func TestThrottleUnknownKindIsNotBlocked(t *testing.T) {
	throttle, clock := newTestThrottle()

	for i := 0; i < 20; i++ {
		if err := throttle.Wait(context.Background(), "jamais-déclaré"); err != nil {
			t.Fatal(err)
		}
	}
	if len(clock.slept) != 0 {
		t.Errorf("un genre non déclaré a été limité : %v", clock.slept)
	}
}

// TestThrottleHonoursCancellation : un import abandonné ne doit pas continuer à
// attendre son tour.
func TestThrottleHonoursCancellation(t *testing.T) {
	throttle, clock := newTestThrottle()
	throttle.SetRate("openlibrary", Rate{Every: time.Hour, Burst: 1})

	if err := throttle.Wait(context.Background(), "openlibrary"); err != nil {
		t.Fatal(err)
	}

	clock.cancel = true
	err := throttle.Wait(context.Background(), "openlibrary")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, attendu context.Canceled", err)
	}
}
