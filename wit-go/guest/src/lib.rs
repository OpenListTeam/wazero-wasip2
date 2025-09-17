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


    fn call_complex_host_func(req: HostRequest) -> String {
        host_log("Guest is calling complex host function 'process-host-request'");
        host_log(&format!("Guest req {:?}", req));
        // Pass the received record directly to the imported host function.
        process_host_request(&req)
    }

        // Implementations for all scalar types
        fn test_u8(v: u8) -> u8 { v }
        fn test_s8(v: i8) -> i8 { v }
        fn test_u16(v: u16) -> u16 { v }
        fn test_s16(v: i16) -> i16 { v }
        fn test_u32(v: u32) -> u32 { v }
        fn test_s32(v: i32) -> i32 { v }
        fn test_u64(v: u64) -> u64 { v }
        fn test_s64(v: i64) -> i64 { v }
        fn test_float32(v: f32) -> f32 { v }
        fn test_float64(v: f64) -> f64 { v }
        fn test_bool(v: bool) -> bool { v }
    
        // Implementations for all option<scalar> types
        fn test_option_u8(v: Option<u8>) -> Option<u8> { v }
        fn test_option_s8(v: Option<i8>) -> Option<i8> { v }
        fn test_option_u16(v: Option<u16>) -> Option<u16> { v }
        fn test_option_s16(v: Option<i16>) -> Option<i16> { v }
        fn test_option_u32(v: Option<u32>) -> Option<u32> { v }
        fn test_option_s32(v: Option<i32>) -> Option<i32> { v }
        fn test_option_u64(v: Option<u64>) -> Option<u64> { v }
        fn test_option_s64(v: Option<i64>) -> Option<i64> { v }
        fn test_option_float32(v: Option<f32>) -> Option<f32> { v }
        fn test_option_float64(v: Option<f64>) -> Option<f64> { v }
        fn test_option_bool(v: Option<bool>) -> Option<bool> { v }

         // Implementations for all result<scalar> types
         fn test_result_u8(v: Result<u8,()>) -> Result<u8,()> { v }
         fn test_result_s8(v: Result<i8,()>) -> Result<i8,()> { v }
         fn test_result_u16(v: Result<u16,()>) -> Result<u16,()> { v }
         fn test_result_s16(v: Result<i16,()>) -> Result<i16,()> { v }
         fn test_result_u32(v: Result<u32,()>) -> Result<u32,()> { v }
         fn test_result_s32(v: Result<i32,()>) -> Result<i32,()> { v }
         fn test_result_u64(v: Result<u64,()>) -> Result<u64,()> { v }
         fn test_result_s64(v: Result<i64,()>) -> Result<i64,()> { v }
         fn test_result_float32(v: Result<f32,()>) -> Result<f32,()> { v }
         fn test_result_float64(v: Result<f64,()>) -> Result<f64,()> { v }
         fn test_result_bool(v: Result<bool,()>) -> Result<bool,()> { v }

          // --- Implementation of the host verification function ---
    fn verify_host_scalars() {
        // Plain scalars
        assert_eq!(host_test_u8(u8::MAX), u8::MAX);
        assert_eq!(host_test_s8(i8::MIN), i8::MIN);
        assert_eq!(host_test_u16(u16::MAX), u16::MAX);
        assert_eq!(host_test_s16(i16::MIN), i16::MIN);
        assert_eq!(host_test_u32(u32::MAX), u32::MAX);
        assert_eq!(host_test_s32(i32::MIN), i32::MIN);
        assert_eq!(host_test_u64(u64::MAX), u64::MAX);
        assert_eq!(host_test_s64(i64::MIN), i64::MIN);
        assert_eq!(host_test_float32(123.456), 123.456);
        assert_eq!(host_test_float64(-987.654), -987.654);
        assert_eq!(host_test_bool(true), true);

        // --- Comprehensive Option<T> tests ---
        // u8
        assert_eq!(host_test_option_u8(Some(u8::MAX)), Some(u8::MAX));
        assert_eq!(host_test_option_u8(None), None);
        // s8
        assert_eq!(host_test_option_s8(Some(i8::MIN)), Some(i8::MIN));
        assert_eq!(host_test_option_s8(None), None);
        // u16
        assert_eq!(host_test_option_u16(Some(u16::MAX)), Some(u16::MAX));
        assert_eq!(host_test_option_u16(None), None);
        // s16
        assert_eq!(host_test_option_s16(Some(i16::MIN)), Some(i16::MIN));
        assert_eq!(host_test_option_s16(None), None);
        // u32
        assert_eq!(host_test_option_u32(Some(u32::MAX)), Some(u32::MAX));
        assert_eq!(host_test_option_u32(None), None);
        // s32
        assert_eq!(host_test_option_s32(Some(i32::MIN)), Some(i32::MIN));
        assert_eq!(host_test_option_s32(None), None);
        // u64
        assert_eq!(host_test_option_u64(Some(u64::MAX)), Some(u64::MAX));
        assert_eq!(host_test_option_u64(None), None);
        // s64
        assert_eq!(host_test_option_s64(Some(i64::MIN)), Some(i64::MIN));
        assert_eq!(host_test_option_s64(None), None);
        // float32
        assert_eq!(host_test_option_float32(Some(123.456)), Some(123.456));
        assert_eq!(host_test_option_float32(None), None);
        // float64
        assert_eq!(host_test_option_float64(Some(-987.654)), Some(-987.654));
        assert_eq!(host_test_option_float64(None), None);
        // bool
        assert_eq!(host_test_option_bool(Some(true)), Some(true));
        assert_eq!(host_test_option_bool(None), None);
    }
}

// Export the implementation.
export!(MyGuest);