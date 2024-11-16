use log::error;

pub enum Operator {
    Eq,
    Ne,
}

pub struct Selector {
    pub key: String,
    pub value: String,
    pub operator: Operator,
}

impl std::fmt::Display for Selector {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            Selector {
                key,
                value,
                operator,
            } => match operator {
                Operator::Eq => write!(f, "{}={}", key, value),
                Operator::Ne => write!(f, "{}!={}", key, value),
            },
        }
    }
}

impl Selector {
    pub fn from(l: &str) -> Selector {
        let not_equal = l.split("!=").collect::<Vec<&str>>();
        let equal_double = l.split("==").collect::<Vec<&str>>();
        let equal_single = l.split('=').collect::<Vec<&str>>();
        let selector = match (not_equal.len(), equal_double.len(), equal_single.len()) {
            (2, _, _) => Selector {
                key: not_equal[0].trim().to_string(),
                value: not_equal[1].trim().to_string(),
                operator: Operator::Ne,
            },
            (_, 2, _) => Selector {
                key: equal_double[0].trim().to_string(),
                value: equal_double[1].trim().to_string(),
                operator: Operator::Eq,
            },
            (_, _, 2) => Selector {
                key: equal_single[0].trim().to_string(),
                value: equal_single[1].trim().to_string(),
                operator: Operator::Eq,
            },
            _ => {
                error!("❌ Invalid label selector format: {}", l);
                panic!("Invalid label selector format");
            }
        };
        if selector.key.is_empty()
            || selector.key.contains('!')
            || selector.key.contains('=')
            || selector.value.is_empty()
            || selector.value.contains('!')
            || selector.value.contains('=')
        {
            error!("❌ Invalid label selector format: {}", l);
            panic!("Invalid label selector format");
        }
        selector
    }
    
}