package core

import (
	"strings"
	"testing"
)

// testUser is a sample type for provider tests.
type testUser struct {
	ID   int
	Name string
}

// testSettings is another type to verify type isolation.
type testSettings struct {
	Theme string
}

// providerConsumerWidget is a stateless widget that captures a value from a provider.
type providerConsumerWidget[T any] struct {
	StatelessBase
	onBuild func(value T, ok bool)
}

func (w providerConsumerWidget[T]) Build(ctx BuildContext) Widget {
	value, ok := ProviderOf[T](ctx)
	if w.onBuild != nil {
		w.onBuild(value, ok)
	}
	return nil
}

func TestInheritedProvider_BasicProvideConsume(t *testing.T) {
	owner := NewBuildOwner()

	user := &testUser{ID: 1, Name: "Alice"}
	var capturedUser *testUser
	var capturedOK bool

	widget := InheritedProvider[*testUser]{
		Value: user,
		Child: providerConsumerWidget[*testUser]{
			onBuild: func(value *testUser, ok bool) {
				capturedUser = value
				capturedOK = ok
			},
		},
	}

	element := newTestInheritedElement(widget, owner)
	element.Mount(nil, nil)

	if !capturedOK {
		t.Fatal("expected ProviderOf to return ok=true")
	}
	if capturedUser != user {
		t.Errorf("expected user %v, got %v", user, capturedUser)
	}
}

func TestInheritedProvider_NotFound(t *testing.T) {
	owner := NewBuildOwner()

	var capturedUser *testUser
	var capturedOK bool

	// Consumer without a provider ancestor
	widget := testStatelessWidget{
		buildFn: func(ctx BuildContext) Widget {
			capturedUser, capturedOK = ProviderOf[*testUser](ctx)
			return nil
		},
	}

	element := newTestStatelessElement(widget, owner)
	element.Mount(nil, nil)

	if capturedOK {
		t.Error("expected ProviderOf to return ok=false when no provider exists")
	}
	if capturedUser != nil {
		t.Errorf("expected nil user, got %v", capturedUser)
	}
}

func TestInheritedProvider_NestedOverride(t *testing.T) {
	owner := NewBuildOwner()

	outerUser := &testUser{ID: 1, Name: "Outer"}
	innerUser := &testUser{ID: 2, Name: "Inner"}
	var capturedUser *testUser

	widget := InheritedProvider[*testUser]{
		Value: outerUser,
		Child: InheritedProvider[*testUser]{
			Value: innerUser,
			Child: providerConsumerWidget[*testUser]{
				onBuild: func(value *testUser, ok bool) {
					capturedUser = value
				},
			},
		},
	}

	element := newTestInheritedElement(widget, owner)
	element.Mount(nil, nil)

	if capturedUser != innerUser {
		t.Errorf("expected inner user %v, got %v", innerUser, capturedUser)
	}
}

func TestInheritedProvider_TypeIsolation(t *testing.T) {
	owner := NewBuildOwner()

	user := &testUser{ID: 1, Name: "Alice"}
	settings := &testSettings{Theme: "dark"}
	var capturedUser *testUser
	var capturedSettings *testSettings
	var userOK, settingsOK bool

	// Provide user, try to consume settings (should fail)
	widget := InheritedProvider[*testUser]{
		Value: user,
		Child: testStatelessWidget{
			buildFn: func(ctx BuildContext) Widget {
				capturedUser, userOK = ProviderOf[*testUser](ctx)
				capturedSettings, settingsOK = ProviderOf[*testSettings](ctx)
				return nil
			},
		},
	}

	element := newTestInheritedElement(widget, owner)
	element.Mount(nil, nil)

	if !userOK || capturedUser != user {
		t.Error("expected to find user provider")
	}
	if settingsOK || capturedSettings != nil {
		t.Error("expected settings to not be found")
	}

	// Now test with both providers
	capturedUser = nil
	capturedSettings = nil

	widget2 := InheritedProvider[*testUser]{
		Value: user,
		Child: InheritedProvider[*testSettings]{
			Value: settings,
			Child: testStatelessWidget{
				buildFn: func(ctx BuildContext) Widget {
					capturedUser, userOK = ProviderOf[*testUser](ctx)
					capturedSettings, settingsOK = ProviderOf[*testSettings](ctx)
					return nil
				},
			},
		},
	}

	element2 := newTestInheritedElement(widget2, owner)
	element2.Mount(nil, nil)

	if !userOK || capturedUser != user {
		t.Error("expected to find user provider")
	}
	if !settingsOK || capturedSettings != settings {
		t.Error("expected to find settings provider")
	}
}

func TestInheritedProvider_CustomShouldNotify(t *testing.T) {
	notifyCalled := false
	var oldValue, newValue *testUser

	oldWidget := InheritedProvider[*testUser]{
		Value: &testUser{ID: 1, Name: "Alice"},
		ShouldNotify: func(old, new *testUser) bool {
			notifyCalled = true
			oldValue = old
			newValue = new
			return old.ID != new.ID // Only notify on ID change
		},
	}

	// Same ID, different name - should not notify
	newWidget := InheritedProvider[*testUser]{
		Value: &testUser{ID: 1, Name: "Alice Updated"},
		ShouldNotify: func(old, new *testUser) bool {
			notifyCalled = true
			oldValue = old
			newValue = new
			return old.ID != new.ID
		},
	}

	shouldNotify := newWidget.UpdateShouldNotify(oldWidget)

	if !notifyCalled {
		t.Error("expected ShouldNotify callback to be called")
	}
	if oldValue.Name != "Alice" {
		t.Errorf("expected old name 'Alice', got %q", oldValue.Name)
	}
	if newValue.Name != "Alice Updated" {
		t.Errorf("expected new name 'Alice Updated', got %q", newValue.Name)
	}
	if shouldNotify {
		t.Error("expected shouldNotify=false when only name changed")
	}

	// Different ID - should notify
	notifyCalled = false
	newWidget2 := InheritedProvider[*testUser]{
		Value: &testUser{ID: 2, Name: "Bob"},
		ShouldNotify: func(old, new *testUser) bool {
			return old.ID != new.ID
		},
	}

	shouldNotify2 := newWidget2.UpdateShouldNotify(oldWidget)

	if !shouldNotify2 {
		t.Error("expected shouldNotify=true when ID changed")
	}
}

func TestInheritedProvider_DefaultComparison(t *testing.T) {
	user := &testUser{ID: 1, Name: "Alice"}

	// Same pointer - should not notify
	oldWidget := InheritedProvider[*testUser]{Value: user}
	newWidget := InheritedProvider[*testUser]{Value: user}

	if newWidget.UpdateShouldNotify(oldWidget) {
		t.Error("expected no notification for same pointer")
	}

	// Different pointer - should notify
	newWidget2 := InheritedProvider[*testUser]{Value: &testUser{ID: 1, Name: "Alice"}}

	if !newWidget2.UpdateShouldNotify(oldWidget) {
		t.Error("expected notification for different pointer")
	}
}

func TestInheritedProvider_ValueType(t *testing.T) {
	// Test with value type (not pointer)
	oldWidget := InheritedProvider[int]{Value: 42}
	sameWidget := InheritedProvider[int]{Value: 42}
	diffWidget := InheritedProvider[int]{Value: 43}

	if sameWidget.UpdateShouldNotify(oldWidget) {
		t.Error("expected no notification for same int value")
	}

	if !diffWidget.UpdateShouldNotify(oldWidget) {
		t.Error("expected notification for different int value")
	}
}

func TestMustProviderOf_Found(t *testing.T) {
	owner := NewBuildOwner()

	user := &testUser{ID: 1, Name: "Alice"}
	var capturedUser *testUser

	widget := InheritedProvider[*testUser]{
		Value: user,
		Child: testStatelessWidget{
			buildFn: func(ctx BuildContext) Widget {
				capturedUser = MustProviderOf[*testUser](ctx)
				return nil
			},
		},
	}

	element := newTestInheritedElement(widget, owner)
	element.Mount(nil, nil)

	if capturedUser != user {
		t.Errorf("expected user %v, got %v", user, capturedUser)
	}
}

func TestMustProviderOf_Panics(t *testing.T) {
	owner := NewBuildOwner()

	var panicValue any

	widget := testStatelessWidget{
		buildFn: func(ctx BuildContext) Widget {
			defer func() {
				panicValue = recover()
			}()
			_ = MustProviderOf[*testUser](ctx)
			return nil
		},
	}

	element := newTestStatelessElement(widget, owner)
	element.Mount(nil, nil)

	if panicValue == nil {
		t.Fatal("expected MustProviderOf to panic when provider not found")
	}

	panicStr, ok := panicValue.(string)
	if !ok {
		t.Fatalf("expected panic value to be string, got %T", panicValue)
	}
	if panicStr == "" {
		t.Error("expected non-empty panic message")
	}
	// Verify the type name is correctly included (not "<nil>")
	if !strings.Contains(panicStr, "*core.testUser") {
		t.Errorf("expected panic message to contain type name '*core.testUser', got %q", panicStr)
	}
}

func TestInheritedProvider_Key(t *testing.T) {
	widget := InheritedProvider[int]{
		Value:     42,
		WidgetKey: "my-key",
	}

	if widget.Key() != "my-key" {
		t.Errorf("expected key 'my-key', got %v", widget.Key())
	}

	widgetNoKey := InheritedProvider[int]{Value: 42}
	if widgetNoKey.Key() != nil {
		t.Errorf("expected nil key, got %v", widgetNoKey.Key())
	}
}

func TestInheritedProvider_ChildWidget(t *testing.T) {
	child := testLeafWidget{id: "test-child"}
	widget := InheritedProvider[int]{
		Value: 42,
		Child: child,
	}

	returnedChild, ok := widget.ChildWidget().(testLeafWidget)
	if !ok {
		t.Fatal("expected ChildWidget() to return testLeafWidget")
	}
	if returnedChild.id != "test-child" {
		t.Errorf("expected child id 'test-child', got %q", returnedChild.id)
	}
}

func TestInheritedProvider_WrongOldWidgetType(t *testing.T) {
	// This tests the edge case where UpdateShouldNotify receives a different widget type
	widget := InheritedProvider[int]{Value: 42}

	// Create a mock inherited widget of different type
	differentWidget := InheritedProvider[string]{Value: "hello"}

	// Should return true (trigger rebuild) when types don't match
	if !widget.UpdateShouldNotify(differentWidget) {
		t.Error("expected UpdateShouldNotify to return true for different widget types")
	}
}
