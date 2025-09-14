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

    fn handle_permissions(p: Permissions) -> Vec<String> {
        let mut result = Vec::new();
        if p.contains(Permissions::READ) {
            result.push("read".to_string());
        }
        if p.contains(Permissions::WRITE) {
            result.push("write".to_string());
        }
        if p.contains(Permissions::EXECUTE) {
            result.push("execute".to_string());
        }
        result
    }

    fn process_users(users: Vec<MyData>) -> Vec<String> {
        let mut names = Vec::new();
        for user in users {
            host_log(&format!("Processing user: id={}, name='{}'", user.a, user.b));
            names.push(user.b);
        }
        names
    }

    fn handle_complex_record(r: ComplexRecord) -> u32 {
        let mut check_sum: u32 = 0;
        check_sum += r.id.len() as u32;

        if let Some(p) = r.permissions {
            if p.contains(Permissions::WRITE) {
                check_sum += 100;
            }
        }
        
        check_sum += r.child_data.len() as u32 * 1000;
        for child in r.child_data {
            check_sum += child.a;
        }

        if let Ok(Shape::Circle(radius)) = r.shape_info {
            check_sum += radius as u32;
        }

        host_log(&format!("Guest processed complex record, checksum: {}", check_sum));
        check_sum
    }

    fn handle_hetero_tuple(t: (u32, u8, String)) -> String {
        format!("Got tuple: ({}, {}, '{}')", t.0, t.1, t.2)
    }

    fn noop_complex(_r: ComplexRecord) {
        // This function does nothing. It's used to benchmark the overhead
        // of lifting the complex-record parameter from the host.
    }
}

// Export the implementation.
export!(MyGuest);