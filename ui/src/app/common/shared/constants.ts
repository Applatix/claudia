export const FIELD_PATTERNS = {
    CIDR: '(^$|\\d{1,3}(\\.\\d{1,3}){3}\\/\\d{1,2}$)',
    ENV_NAME: '^[A-Za-z0-9]([-A-Za-z0-9_]*)?[A-Za-z0-9]$',
    // Password will match only: 8+ letters, at least 1 lower case letter,
    // at least 1 upper case letter, and at least 1 special character
    PASSWORD: '^(?=.*?[A-Za-z])(?=.*?[0-9])(?=.*?[#?!@$%^&*-]).{8,}$',
    // Email regex
    EMAIL: '^[a-z0-9]+(\.[_a-z0-9]+)*@[a-z0-9-]+(\.[a-z0-9-]+)*(\.[a-z]{2,15})$',
};

export const ERROR_CODES = {
    INTERNAL_ERROR: 'INTERNAL_ERROR',
    INVALID_REQUEST: 'INVALID_REQUEST',
    UNAUTHENTICATED_REQUEST: 'UNAUTHENTICATED_REQUEST',
    UNAUTHORIZED_REQUEST: 'UNAUTHORIZED_REQUEST',
    ENTITY_NOT_FOUND: 'ENTITY_NOT_FOUND',
};

export interface AppError {
    code?: string;

    message?: string;
}
