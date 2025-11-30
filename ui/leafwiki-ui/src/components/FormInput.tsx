import { Input } from '@/components/ui/input'

type FormInputProps = {
  label?: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  testid?: string
  error?: string
  type?: string
  autoFocus?: boolean
  readOnly?: boolean
}

export function FormInput({
  label,
  value,
  autoFocus,
  onChange,
  testid,
  placeholder,
  error,
  type = 'text',
  readOnly = false,
}: FormInputProps) {
  return (
    <div className="form-input">
      {label && <label className="form-input__label">{label}</label>}
      <Input
        autoFocus={autoFocus || false}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        readOnly={readOnly}
        className={error ? 'form-input__input-error' : ''}
        data-testid={testid}
      />
      {error && <p className="form-input__error">{error}</p>}
    </div>
  )
}
