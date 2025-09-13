wit_bindgen::generate!({
    path: "test.wit",
    world: "test-world",
});

use std::cell::RefCell;

// Use a static variable to store state between calls for testing purposes.
thread_local! {
    static GUEST_STATE: RefCell<String> = RefCell::new(String::new());
}

// Define the main struct that will implement the WIT world.
struct MyGuest;

// Implement the `exports` specified in the WIT file.
impl Guest for MyGuest {
    fn process_string(s: String) {
        // Log the received string via an imported host function.
        host_log(&format!("Guest received string: '{}'", s));
        // Store the string in our static state.
        GUEST_STATE.with(|state| {
            *state.borrow_mut() = s;
        });
    }

    fn roundtrip_string(s: String) -> String {
        // Return the string with a prefix added by the guest.
        format!("Guest says: {}", s)
    }

    fn create_data() -> MyData {
        // Create and return a sample record.
        MyData {
            a: 123,
            b: "hello from guest".to_string(),
            c: vec![10, 20, 30],
        }
    }


    fn handle_option(opt: Option<String>) -> Option<u32> {
        match opt {
            Some(s) => {
                host_log(&format!("Guest got Some: '{}'", s));
                Some(s.len() as u32)
            }
            None => {
                host_log("Guest got None");
                None
            }
        }
    }

    fn handle_result(res: Result<String, Color>) -> Result<u32, Color> {
        match res {
            Ok(s) => {
                host_log(&format!("Guest got Ok: '{}'", s));
                Ok(s.len() as u32)
            }
            Err(c) => {
                let color_str = match c {
                    Color::Red => "Red",
                    Color::Green => "Green",
                    Color::Blue => "Blue",
                };
                host_log(&format!("Guest got Err: {}", color_str));
                Err(c)
            }
        }
    }

    fn handle_variant(s: Shape) -> String {
        match s {
            Shape::Circle(radius) => format!("Circle with radius {}", radius),
            Shape::Rect(dims) => format!("Rectangle with size {}x{}", dims.0, dims.1),
        }
    }
}

// Export the implementation.
export!(MyGuest);