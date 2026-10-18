#pragma once
#include "platform/windows.hpp"
#if defined(__MINGW32__) && !defined(_Check_return_opt_)
#define _Check_return_opt_
#endif
#if defined(__MINGW32__) && !defined(_Post_ptr_invalid_)
#define _Post_ptr_invalid_
#endif
#if defined(__MINGW32__) && !defined(_Frees_ptr_opt_)
#define _Frees_ptr_opt_
#endif
#include <leechcore.h>
